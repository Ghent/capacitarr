package poller

import (
	"testing"
	"time"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"

	_ "github.com/ncruces/go-sqlite3/embed" // load the embedded SQLite WASM binary
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupPollerTestDB creates an in-memory SQLite database with migrations applied,
// seeds default preferences, and returns the database and a service registry.
func setupPollerTestDB(t *testing.T) (*gorm.DB, *services.Registry) {
	t.Helper()

	database, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	sqlDB.SetMaxOpenConns(1)

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	pref := db.PreferenceSet{
		ID:                    1,
		ExecutionMode:         "dry-run",
		LogLevel:              "info",
		AuditLogRetentionDays: 30,
		PollIntervalSeconds:   300,
	}
	if err := database.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		t.Fatalf("Failed to seed preferences: %v", err)
	}

	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cfg := &config.Config{JWTSecret: "test"}
	reg := services.NewRegistry(database, bus, cfg)
	reg.InitVersion("v0.0.0-test")

	return database, reg
}

// ---------- getPollInterval() ----------

func TestGetPollInterval_Default(t *testing.T) {
	_, reg := setupPollerTestDB(t)
	p := New(reg)

	interval := p.getPollInterval()
	expected := 5 * time.Minute
	if interval != expected {
		t.Errorf("Expected default interval %v, got %v", expected, interval)
	}
}

func TestGetPollInterval_BelowMinimum(t *testing.T) {
	database, reg := setupPollerTestDB(t)
	p := New(reg)

	// Set poll interval to 10s (below minimum of 30s)
	database.Model(&db.PreferenceSet{}).Where("id = 1").Update("poll_interval_seconds", 10)

	interval := p.getPollInterval()
	expected := 5 * time.Minute // falls back to 300s (5 min)
	if interval != expected {
		t.Errorf("Expected fallback interval %v for below-minimum value, got %v", expected, interval)
	}
}

func TestGetPollInterval_CustomValue(t *testing.T) {
	database, reg := setupPollerTestDB(t)
	p := New(reg)

	// Set poll interval to 60s
	database.Model(&db.PreferenceSet{}).Where("id = 1").Update("poll_interval_seconds", 60)

	interval := p.getPollInterval()
	expected := 1 * time.Minute
	if interval != expected {
		t.Errorf("Expected interval %v, got %v", expected, interval)
	}
}

// ---------- Start()/Stop() lifecycle ----------

func TestStartStop_Lifecycle(t *testing.T) {
	_, reg := setupPollerTestDB(t)
	p := New(reg)

	// Start should not panic
	p.Start()

	// Give the goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop should not panic
	p.Stop()
}

// ---------- safePoll() ----------

func TestSafePoll_NoPanic(t *testing.T) {
	_, reg := setupPollerTestDB(t)
	p := New(reg)

	// safePoll on a poller with no integrations should complete without panic.
	// It will attempt to poll but find no enabled integrations, which is safe.
	p.safePoll()
}

// ---------- normalizePath() ----------

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "unix path unchanged", in: "/media/movies", want: "/media/movies"},
		{name: "unix root unchanged", in: "/", want: "/"},
		{name: "empty string", in: "", want: ""},
		{name: "windows backslash", in: `H:\Movies`, want: "H:/Movies"},
		{name: "windows drive root", in: `H:\`, want: "H:/"},
		{name: "windows deep path with spaces", in: `H:\User\Google Movie HDD\Deluge Movie HDD`, want: "H:/User/Google Movie HDD/Deluge Movie HDD"},
		{name: "windows UNC path", in: `\\server\share\movies`, want: "//server/share/movies"},
		{name: "mixed separators", in: `H:\media/movies\subfolder`, want: "H:/media/movies/subfolder"},
		{name: "already forward slashes on Windows", in: "H:/Movies", want: "H:/Movies"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePath(tt.in)
			if got != tt.want {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------- findMediaMounts() ----------

func TestFindMediaMounts_UnixPaths(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		"/media": {Path: "/media", TotalBytes: 1000, FreeBytes: 500},
	}
	rootFolders := map[string]bool{
		"/media/movies": true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if !mounts["/media"] {
		t.Errorf("expected /media to be a media mount, got %v", mounts)
	}
	if len(mounts) != 1 {
		t.Errorf("expected 1 mount, got %d", len(mounts))
	}
}

func TestFindMediaMounts_UnixRootFallback(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		"/": {Path: "/", TotalBytes: 1000, FreeBytes: 500},
	}
	rootFolders := map[string]bool{
		"/data/movies": true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if !mounts["/"] {
		t.Errorf("expected / to be a media mount when it's the only option, got %v", mounts)
	}
}

func TestFindMediaMounts_MostSpecificWins(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		"/":      {Path: "/", TotalBytes: 5000, FreeBytes: 2500},
		"/media": {Path: "/media", TotalBytes: 1000, FreeBytes: 500},
	}
	rootFolders := map[string]bool{
		"/media/movies": true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if mounts["/"] {
		t.Errorf("did not expect / to be selected (more specific /media exists)")
	}
	if !mounts["/media"] {
		t.Errorf("expected /media to be selected as most specific mount, got %v", mounts)
	}
}

func TestFindMediaMounts_WindowsDriveRoot(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		`G:\`: {Path: `G:\`, TotalBytes: 1000, FreeBytes: 500},
	}
	rootFolders := map[string]bool{
		`G:\Movies`: true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if !mounts[`G:\`] {
		t.Errorf("expected G:\\ to be a media mount, got %v", mounts)
	}
}

func TestFindMediaMounts_WindowsDeepPathWithSpaces(t *testing.T) {
	// This is the exact scenario from the user's bug report:
	// Radarr on Windows with Google Drive, root folder deep in H:\
	diskMap := map[string]integrations.DiskSpace{
		`H:\`: {Path: `H:\`, TotalBytes: 2000000000000, FreeBytes: 500000000000},
	}
	rootFolders := map[string]bool{
		`H:\User\Google Movie HDD\Deluge Movie HDD`: true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if !mounts[`H:\`] {
		t.Errorf("expected H:\\ to be a media mount for deep Google Drive path, got %v", mounts)
	}
}

func TestFindMediaMounts_WindowsMostSpecificWins(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		`C:\`:      {Path: `C:\`, TotalBytes: 500, FreeBytes: 100},
		`C:\Media`: {Path: `C:\Media`, TotalBytes: 1000, FreeBytes: 500},
	}
	rootFolders := map[string]bool{
		`C:\Media\Movies`: true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if mounts[`C:\`] {
		t.Errorf("did not expect C:\\ to be selected (more specific C:\\Media exists)")
	}
	if !mounts[`C:\Media`] {
		t.Errorf("expected C:\\Media to be the most specific mount, got %v", mounts)
	}
}

func TestFindMediaMounts_WindowsUNCPath(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		`\\server\share`: {Path: `\\server\share`, TotalBytes: 1000, FreeBytes: 500},
	}
	rootFolders := map[string]bool{
		`\\server\share\movies`: true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if !mounts[`\\server\share`] {
		t.Errorf("expected UNC path \\\\server\\share to be a media mount, got %v", mounts)
	}
}

func TestFindMediaMounts_NoMatch(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		"/media": {Path: "/media", TotalBytes: 1000, FreeBytes: 500},
	}
	rootFolders := map[string]bool{
		"/data/movies": true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if len(mounts) != 0 {
		t.Errorf("expected 0 mounts when no match exists, got %v", mounts)
	}
}

func TestFindMediaMounts_ExactMatchRootFolder(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		"/media/movies": {Path: "/media/movies", TotalBytes: 1000, FreeBytes: 500},
	}
	rootFolders := map[string]bool{
		"/media/movies": true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if !mounts["/media/movies"] {
		t.Errorf("expected exact mount match, got %v", mounts)
	}
}

func TestFindMediaMounts_WindowsExactMatch(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		`D:\Serenity Collection`: {Path: `D:\Serenity Collection`, TotalBytes: 1000, FreeBytes: 500},
	}
	rootFolders := map[string]bool{
		`D:\Serenity Collection`: true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if !mounts[`D:\Serenity Collection`] {
		t.Errorf("expected exact Windows path match, got %v", mounts)
	}
}

func TestFindMediaMounts_MultipleRootFoldersSameDrive(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		`H:\`: {Path: `H:\`, TotalBytes: 2000, FreeBytes: 1000},
	}
	rootFolders := map[string]bool{
		`H:\Movies`:   true,
		`H:\TV Shows`: true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if !mounts[`H:\`] {
		t.Errorf("expected H:\\ to match both root folders, got %v", mounts)
	}
	if len(mounts) != 1 {
		t.Errorf("expected 1 mount (both root folders on same drive), got %d", len(mounts))
	}
}

func TestFindMediaMounts_RootPrunedWhenMoreSpecificExists(t *testing.T) {
	diskMap := map[string]integrations.DiskSpace{
		"/":      {Path: "/", TotalBytes: 5000, FreeBytes: 2500},
		"/media": {Path: "/media", TotalBytes: 1000, FreeBytes: 500},
		"/data":  {Path: "/data", TotalBytes: 1000, FreeBytes: 500},
	}
	rootFolders := map[string]bool{
		"/media/movies": true,
		"/data/tv":      true,
	}

	mounts := findMediaMounts(diskMap, rootFolders)

	if mounts["/"] {
		t.Errorf("did not expect / to survive when more specific mounts matched")
	}
	if !mounts["/media"] || !mounts["/data"] {
		t.Errorf("expected /media and /data, got %v", mounts)
	}
}
