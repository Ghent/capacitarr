package poller

import (
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

const (
	e2eMount      = "/data"
	e2eCollection = "Sonic the Hedgehog Collection"
	e2eTotalBytes = int64(100_000_000_000) // 100 GB
	e2eFreeBytes  = int64(5_000_000_000)   // 5 GB free → 95% used
	e2eMemberSize = int64(10_000_000_000)  // 10 GB
	e2eDecoySize  = int64(1_000_000_000)   // 1 GB
	e2eSoloSize   = int64(30_000_000_000)  // 30 GB
)

// fakeArr is an in-memory MediaSource + DiskReporter + MediaDeleter used to
// drive the poll → enrich → threshold → QueueFromEngine → Delete path.
type fakeArr struct {
	items   []integrations.MediaItem
	disks   []integrations.DiskSpace
	folders []string

	mu      sync.Mutex
	deleted []string
}

func (f *fakeArr) TestConnection() error { return nil }

func (f *fakeArr) GetMediaItems() ([]integrations.MediaItem, error) {
	out := make([]integrations.MediaItem, len(f.items))
	copy(out, f.items)
	return out, nil
}

func (f *fakeArr) GetDiskSpace() ([]integrations.DiskSpace, error) {
	out := make([]integrations.DiskSpace, len(f.disks))
	copy(out, f.disks)
	return out, nil
}

func (f *fakeArr) GetRootFolders() ([]string, error) {
	out := make([]string, len(f.folders))
	copy(out, f.folders)
	return out, nil
}

func (f *fakeArr) DeleteMediaItem(item integrations.MediaItem, _ integrations.DeleteOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, item.Title)
	return nil
}

func (f *fakeArr) deletedTitles() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.deleted))
	copy(out, f.deleted)
	return out
}

type e2eFixture struct {
	reg  *services.Registry
	p    *Poller
	fake *fakeArr
}

func setupPollerE2E(t *testing.T, collectionDeletion bool, items []integrations.MediaItem) *e2eFixture {
	t.Helper()

	database, reg := setupPollerTestDB(t)

	if err := database.Model(&db.PreferenceSet{}).Where("id = 1").Updates(map[string]any{
		"default_disk_group_mode": db.ModeAuto,
		"deletions_enabled":       true,
	}).Error; err != nil {
		t.Fatalf("set auto prefs: %v", err)
	}

	defaults := make([]db.FactorDefault, 0, len(engine.DefaultFactors()))
	for _, f := range engine.DefaultFactors() {
		defaults = append(defaults, db.FactorDefault{Key: f.Key(), DefaultWeight: f.DefaultWeight()})
	}
	db.SeedFactorWeights(database, defaults)

	reg.Deletion.SetTestPace(rate.NewLimiter(rate.Inf, 1), 0)

	fake := &fakeArr{
		items:   items,
		folders: []string{e2eMount + "/movies"},
		disks: []integrations.DiskSpace{{
			Path:       e2eMount,
			TotalBytes: e2eTotalBytes,
			FreeBytes:  e2eFreeBytes,
		}},
	}

	cfg, err := reg.Integration.Create(db.IntegrationConfig{
		Type:               string(integrations.IntegrationTypeRadarr),
		Name:               "E2E Radarr",
		URL:                "http://fake-radarr.test",
		APIKey:             "test-api-key",
		Enabled:            true,
		CollectionDeletion: collectionDeletion,
		AddImportExclusion: true,
	})
	if err != nil {
		t.Fatalf("create integration: %v", err)
	}
	for i := range fake.items {
		fake.items[i].IntegrationID = cfg.ID
	}
	reg.Integration.SetTestClient(cfg.ID, fake)

	reg.Deletion.Start()
	t.Cleanup(reg.Deletion.Stop)

	return &e2eFixture{reg: reg, p: New(reg), fake: fake}
}

func waitForDeletes(t *testing.T, fake *fakeArr, want int) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got := fake.deletedTitles()
		if len(got) == want {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}
	got := fake.deletedTitles()
	t.Fatalf("timeout waiting for %d deletes, got %d: %v", want, len(got), got)
	return got
}

func e2eMovie(title, externalID string, size int64, collections ...string) integrations.MediaItem {
	item := integrations.MediaItem{
		ExternalID: externalID,
		Type:       integrations.MediaTypeMovie,
		Title:      title,
		SizeBytes:  size,
		Path:       e2eMount + "/movies/" + title,
		Rating:     3.0,
	}
	if len(collections) > 0 {
		item.Collections = append([]string(nil), collections...)
	}
	return item
}

// TestPoller_AutoModeE2E is the single integration test for the live deletion
// path. Auto-mode evaluate tests historically passed a nil registry so
// dispatchByMode no-oped; this test registers a real fake *arr and goes
// through poll() → QueueFromEngine → the deletion worker.
func TestPoller_AutoModeE2E(t *testing.T) {
	t.Run("overThreshold_autoQueuesAndDeletes", func(t *testing.T) {
		fx := setupPollerE2E(t, false, []integrations.MediaItem{
			e2eMovie("Serenity", "movie-1", e2eSoloSize),
		})

		fx.p.poll()

		got := waitForDeletes(t, fx.fake, 1)
		if got[0] != "Serenity" {
			t.Errorf("deleted %v, want [Serenity]", got)
		}
	})

	t.Run("collectionExpansion_wrongMembersFail", func(t *testing.T) {
		fx := setupPollerE2E(t, true, []integrations.MediaItem{
			e2eMovie("Sonic the Hedgehog", "sonic-1", e2eMemberSize, e2eCollection),
			e2eMovie("Sonic the Hedgehog 2", "sonic-2", e2eMemberSize, e2eCollection),
			e2eMovie("Sonic the Hedgehog 3", "sonic-3", e2eMemberSize, e2eCollection),
			e2eMovie("Unrelated Decoy", "decoy-1", e2eDecoySize),
		})

		fx.p.poll()

		got := waitForDeletes(t, fx.fake, 3)
		sort.Strings(got)
		want := []string{"Sonic the Hedgehog", "Sonic the Hedgehog 2", "Sonic the Hedgehog 3"}
		if len(got) != len(want) {
			t.Fatalf("deleted %v, want exactly %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("deleted %v, want exactly %v", got, want)
			}
		}
		for _, title := range got {
			if title == "Unrelated Decoy" {
				t.Fatal("collection expansion included the decoy — wrong members")
			}
		}
	})

	t.Run("listSnoozedKeysError_failClosed", func(t *testing.T) {
		fx := setupPollerE2E(t, false, []integrations.MediaItem{
			e2eMovie("Serenity", "movie-1", e2eSoloSize),
		})
		fx.reg.Approval.SetListSnoozedKeysError(errors.New("snooze lookup failed"))

		ch := fx.reg.Bus.Subscribe()
		defer fx.reg.Bus.Unsubscribe(ch)

		fx.p.poll()

		if fx.reg.Deletion.QueueLen() != 0 {
			t.Errorf("expected empty deletion queue on snooze lookup error, got %d", fx.reg.Deletion.QueueLen())
		}

		deadline := time.Now().Add(150 * time.Millisecond)
		for time.Now().Before(deadline) {
			if len(fx.fake.deletedTitles()) != 0 {
				t.Fatalf("fail-closed broke: deleted %v", fx.fake.deletedTitles())
			}
			time.Sleep(5 * time.Millisecond)
		}

		gotError := false
		drain := time.After(50 * time.Millisecond)
	drainLoop:
		for {
			select {
			case evt := <-ch:
				if _, ok := evt.(events.EngineErrorEvent); ok {
					gotError = true
				}
			case <-drain:
				break drainLoop
			}
		}
		if !gotError {
			t.Error("expected EngineErrorEvent when ListSnoozedKeys fails")
		}
	})
}
