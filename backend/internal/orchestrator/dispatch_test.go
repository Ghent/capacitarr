package orchestrator

import (
	"sort"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

type stubDeletion struct {
	calls  int
	failAt int
}

func (s *stubDeletion) QueueFromEngine(_ services.EngineDeleteRequest) error {
	s.calls++
	if s.failAt > 0 && s.calls >= s.failAt {
		return services.ErrDeletionQueueFull
	}
	return nil
}

type stubApproval struct {
	cleared int
}

func (s *stubApproval) ClearQueueForDiskGroup(uint) (int, error) {
	s.cleared++
	return 0, nil
}
func (s *stubApproval) ListSnoozedKeys(uint) (map[string]bool, error) {
	return map[string]bool{}, nil
}
func (s *stubApproval) BulkUpsertPending([]db.ApprovalQueueItem) (int, int, error) {
	return 0, 0, nil
}
func (s *stubApproval) ReconcileQueue(uint, map[string]bool) (int, error) { return 0, nil }

type stubPublisher struct {
	events []events.Event
}

func (s *stubPublisher) Publish(event events.Event) {
	s.events = append(s.events, event)
}

func twoGBItem(title string) integrations.MediaItem {
	return integrations.MediaItem{
		Title:     title,
		Type:      integrations.MediaTypeMovie,
		Path:      "/data/" + title,
		SizeBytes: 2 * 1024 * 1024 * 1024,
	}
}

func TestDispatchFiltered_StopsOnQueueFull(t *testing.T) {
	del := &stubDeletion{failAt: 2}
	o := New(Deps{
		Approval: &stubApproval{},
		Deletion: del,
		Bus:      &stubPublisher{},
	})
	ectx := &evaluationContext{
		group: db.DiskGroup{ID: 1, Mode: db.ModeDryRun, MountPath: "/data"},
		groupAcc: &GroupAccumulator{
			MountPath: "/data",
			Mode:      db.ModeDryRun,
		},
		expandedCollections:    make(map[string]bool),
		integrationConfigCache: make(map[uint]*db.IntegrationConfig),
		snoozedKeys:            map[string]bool{},
	}
	filtered := []engine.EvaluatedItem{
		{Item: twoGBItem("A"), Score: 0.9},
		{Item: twoGBItem("B"), Score: 0.8},
		{Item: twoGBItem("C"), Score: 0.7},
		{Item: twoGBItem("D"), Score: 0.6},
	}

	queued := o.dispatchFiltered(ectx, filtered, skipStats{}, 8*1024*1024*1024)
	if queued != 1 {
		t.Errorf("queued = %d, want 1", queued)
	}
	if del.calls != 2 {
		t.Errorf("QueueFromEngine calls = %d, want 2 (stop after first full)", del.calls)
	}
	if !ectx.queueFull {
		t.Error("expected queueFull to be set")
	}
	if ectx.groupAcc.QueueFullSkipped != 3 {
		t.Errorf("QueueFullSkipped = %d, want 3 (failed item + remaining 2)", ectx.groupAcc.QueueFullSkipped)
	}
	if ectx.groupAcc.Candidates != 1 {
		t.Errorf("Candidates = %d, want 1", ectx.groupAcc.Candidates)
	}
}

func TestEvaluateDiskGroup_QueueFullStopsDispatch(t *testing.T) {
	del := &stubDeletion{failAt: 2}
	bus := &stubPublisher{}
	o := New(Deps{
		Approval: &stubApproval{},
		Deletion: del,
		Bus:      bus,
	})

	const gib int64 = 1024 * 1024 * 1024
	group := db.DiskGroup{
		ID:           1,
		MountPath:    "/data",
		TotalBytes:   10 * gib,
		UsedBytes:    9 * gib,
		ThresholdPct: 80,
		TargetPct:    20,
		Mode:         db.ModeDryRun,
	}
	items := []integrations.MediaItem{
		twoGBItem("One"),
		twoGBItem("Two"),
		twoGBItem("Three"),
		twoGBItem("Four"),
		twoGBItem("Five"),
	}
	acc := NewRunAccumulator()
	evalCtx := &engine.EvaluationContext{
		ActiveIntegrationTypes: map[integrations.IntegrationType]bool{},
	}

	queued := o.EvaluateDiskGroup(
		acc, group, items, integrations.NewIntegrationRegistry(), 1,
		db.PreferenceSet{TiebreakerMethod: db.TiebreakerSizeDesc},
		map[string]int{"file_size": 10},
		nil,
		evalCtx,
	)
	if queued != 1 {
		t.Errorf("queued = %d, want 1", queued)
	}
	if del.calls != 2 {
		t.Errorf("QueueFromEngine calls = %d, want 2", del.calls)
	}
	ga := acc.Groups[group.ID]
	if ga == nil {
		t.Fatal("expected group accumulator")
	}
	if ga.QueueFullSkipped == 0 {
		t.Error("expected QueueFullSkipped > 0")
	}
}

type capturingDeletion struct {
	titles []string
}

func (s *capturingDeletion) QueueFromEngine(req services.EngineDeleteRequest) error {
	s.titles = append(s.titles, req.Item.Title)
	return nil
}

type capturingSunset struct {
	titles []string
	items  []db.SunsetQueueItem
}

func (s *capturingSunset) ListSunsettedKeys(uint) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (s *capturingSunset) BulkQueueSunset(items []db.SunsetQueueItem, _ services.SunsetDeps) (int, error) {
	for _, it := range items {
		s.titles = append(s.titles, it.MediaName)
		s.items = append(s.items, it)
	}
	return len(items), nil
}

func (s *capturingSunset) Escalate(uint, int64, services.SunsetDeps) (int64, int, error) {
	return 0, 0, nil
}

type snoozeApproval struct {
	stubApproval
	keys map[string]bool
}

func (s *snoozeApproval) ListSnoozedKeys(uint) (map[string]bool, error) {
	if s.keys == nil {
		return map[string]bool{}, nil
	}
	return s.keys, nil
}

type stubIntegrations struct {
	collectionDeletion bool
}

func (s *stubIntegrations) GetByID(id uint) (*db.IntegrationConfig, error) {
	return &db.IntegrationConfig{ID: id, CollectionDeletion: s.collectionDeletion}, nil
}

func sameCandidateLibrary() (db.DiskGroup, []integrations.MediaItem, db.PreferenceSet, map[string]int, *engine.EvaluationContext) {
	const gib int64 = 1024 * 1024 * 1024
	sunsetPct := 70.0
	group := db.DiskGroup{
		ID:           1,
		MountPath:    "/data",
		TotalBytes:   100 * gib,
		UsedBytes:    90 * gib, // 90% — above dry-run evaluateAt (85) and sunset evaluateAt (70)
		ThresholdPct: 85,
		TargetPct:    75,
		SunsetPct:    &sunsetPct,
	}
	items := []integrations.MediaItem{
		{
			Title: "Serenity", Type: integrations.MediaTypeMovie,
			Path: "/data/Serenity", SizeBytes: 2 * gib,
			IntegrationID: 1, ExternalID: "m-serenity", Rating: 3,
		},
		{
			Title: "Firefly The Movie", Type: integrations.MediaTypeMovie,
			Path: "/data/Firefly The Movie", SizeBytes: 2 * gib,
			IntegrationID: 1, ExternalID: "m-ftm", Rating: 3,
		},
		{
			Title: "Firefly", Type: integrations.MediaTypeShow,
			Path: "/data/Firefly", SizeBytes: 2 * gib,
			IntegrationID: 1, ExternalID: "s-firefly", Rating: 3,
		},
		{
			Title: "Firefly - Season 1", Type: integrations.MediaTypeSeason, ShowTitle: "Firefly",
			Path: "/data/Firefly/S1", SizeBytes: 1 * gib,
			IntegrationID: 1, ExternalID: "s-firefly-1", Rating: 3,
		},
		{
			Title: "Firefly - Season 2", Type: integrations.MediaTypeSeason, ShowTitle: "Firefly",
			Path: "/data/Firefly/S2", SizeBytes: 1 * gib,
			IntegrationID: 1, ExternalID: "s-firefly-2", Rating: 3,
		},
		{
			Title: "Skip Me", Type: integrations.MediaTypeMovie,
			Path: "/data/Skip Me", SizeBytes: 2 * gib,
			IntegrationID: 1, ExternalID: "m-skip", Rating: 3,
		},
		{
			Title: "The Avengers", Type: integrations.MediaTypeMovie,
			Path: "/data/The Avengers", SizeBytes: 1 * gib,
			IntegrationID: 1, ExternalID: "m-avengers", Rating: 3,
			Collections: []string{"MCU"}, CollectionSources: map[string]uint{"MCU": 1},
		},
		{
			Title: "Iron Man", Type: integrations.MediaTypeMovie,
			Path: "/data/Iron Man", SizeBytes: 1 * gib,
			IntegrationID: 1, ExternalID: "m-ironman", Rating: 3,
			Collections: []string{"MCU"}, CollectionSources: map[string]uint{"MCU": 1},
		},
	}
	prefs := db.PreferenceSet{TiebreakerMethod: db.TiebreakerSizeDesc, SunsetDays: 30}
	weights := map[string]int{"file_size": 10}
	evalCtx := &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}}
	return group, items, prefs, weights, evalCtx
}

func sortedCopy(titles []string) []string {
	out := append([]string(nil), titles...)
	sort.Strings(out)
	return out
}

// TestEvaluateDiskGroup_DryRunAndSunsetAdmitSameTitles is the slice D
// acceptance test: one fixture library, shared score → filter → expand,
// dry-run and sunset admit the same titles. Show-level Firefly is deduped
// (seasons exist), Skip Me is snoozed, MCU expands on both presets.
func TestEvaluateDiskGroup_DryRunAndSunsetAdmitSameTitles(t *testing.T) {
	group, items, prefs, weights, evalCtx := sameCandidateLibrary()
	approval := &snoozeApproval{keys: map[string]bool{
		db.MediaKey("Skip Me", string(integrations.MediaTypeMovie)): true,
	}}
	integrationsSvc := &stubIntegrations{collectionDeletion: true}
	registry := integrations.NewIntegrationRegistry()

	dryDel := &capturingDeletion{}
	dryGroup := group
	dryGroup.Mode = db.ModeDryRun
	dryOrch := New(Deps{
		Approval:     approval,
		Deletion:     dryDel,
		Integrations: integrationsSvc,
		Bus:          &stubPublisher{},
	})
	dryOrch.EvaluateDiskGroup(NewRunAccumulator(), dryGroup, items, registry, 1, prefs, weights, nil, evalCtx)

	sunCap := &capturingSunset{}
	sunGroup := group
	sunGroup.Mode = db.ModeSunset
	sunOrch := New(Deps{
		Approval:     approval,
		Deletion:     &stubDeletion{},
		Integrations: integrationsSvc,
		Sunset:       sunCap,
		Bus:          &stubPublisher{},
	})
	sunOrch.EvaluateDiskGroup(NewRunAccumulator(), sunGroup, items, registry, 1, prefs, weights, nil, evalCtx)

	dryTitles := sortedCopy(dryDel.titles)
	sunTitles := sortedCopy(sunCap.titles)
	if len(dryTitles) == 0 {
		t.Fatal("dry-run admitted no titles")
	}
	if len(sunTitles) == 0 {
		t.Fatal("sunset admitted no titles")
	}
	if len(dryTitles) != len(sunTitles) {
		t.Fatalf("admitted count differs: dry-run %v sunset %v", dryTitles, sunTitles)
	}
	for i := range dryTitles {
		if dryTitles[i] != sunTitles[i] {
			t.Fatalf("admitted titles differ at %d: dry-run %v sunset %v", i, dryTitles, sunTitles)
		}
	}

	admitted := map[string]bool{}
	for _, title := range dryTitles {
		admitted[title] = true
	}
	if admitted["Firefly"] {
		t.Error("show-level Firefly should be deduped when seasons exist")
	}
	if admitted["Skip Me"] {
		t.Error("snoozed Skip Me should not be admitted")
	}
	if !admitted["The Avengers"] || !admitted["Iron Man"] {
		t.Errorf("collection expand should admit both MCU members, got %v", dryTitles)
	}

	var sawCollectionGroup bool
	for _, item := range sunCap.items {
		if item.MediaName == "The Avengers" || item.MediaName == "Iron Man" {
			if item.CollectionGroup != "MCU" {
				t.Errorf("%s CollectionGroup = %q, want MCU", item.MediaName, item.CollectionGroup)
			}
			sawCollectionGroup = true
		}
	}
	if !sawCollectionGroup {
		t.Error("expected sunset holds for expanded MCU members")
	}
}

func TestEvaluateDiskGroup_SunsetMisconfigured(t *testing.T) {
	bus := &stubPublisher{}
	o := New(Deps{
		Approval: &stubApproval{},
		Deletion: &stubDeletion{},
		Sunset:   &capturingSunset{},
		Bus:      bus,
	})
	const gib int64 = 1024 * 1024 * 1024
	group := db.DiskGroup{
		ID:           1,
		MountPath:    "/data",
		TotalBytes:   100 * gib,
		UsedBytes:    90 * gib,
		ThresholdPct: 85,
		TargetPct:    75,
		Mode:         db.ModeSunset,
	}
	queued := o.EvaluateDiskGroup(
		NewRunAccumulator(), group, []integrations.MediaItem{twoGBItem("One")},
		integrations.NewIntegrationRegistry(), 1,
		db.PreferenceSet{}, map[string]int{"file_size": 10}, nil,
		&engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}},
	)
	if queued != 0 {
		t.Errorf("queued = %d, want 0", queued)
	}
	if len(bus.events) != 1 {
		t.Fatalf("events = %d, want 1 SunsetMisconfigured", len(bus.events))
	}
	if _, ok := bus.events[0].(events.SunsetMisconfiguredEvent); !ok {
		t.Errorf("event type %T, want SunsetMisconfiguredEvent", bus.events[0])
	}
}

func TestEvaluateDiskGroup_SunsetBelowEvaluateAtKeepsHolds(t *testing.T) {
	approval := &stubApproval{}
	sun := &capturingSunset{}
	o := New(Deps{
		Approval: approval,
		Deletion: &stubDeletion{},
		Sunset:   sun,
		Bus:      &stubPublisher{},
	})
	const gib int64 = 1024 * 1024 * 1024
	sunsetPct := 70.0
	group := db.DiskGroup{
		ID:           1,
		MountPath:    "/data",
		TotalBytes:   100 * gib,
		UsedBytes:    50 * gib,
		ThresholdPct: 85,
		TargetPct:    75,
		SunsetPct:    &sunsetPct,
		Mode:         db.ModeSunset,
	}
	queued := o.EvaluateDiskGroup(
		NewRunAccumulator(), group, []integrations.MediaItem{twoGBItem("One")},
		integrations.NewIntegrationRegistry(), 1,
		db.PreferenceSet{SunsetDays: 30}, map[string]int{"file_size": 10}, nil,
		&engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}},
	)
	if queued != 0 {
		t.Errorf("queued = %d, want 0", queued)
	}
	if len(sun.titles) != 0 {
		t.Errorf("sunset admitted %v, want none below evaluateAt", sun.titles)
	}
	if approval.cleared != 0 {
		t.Errorf("ClearQueueForDiskGroup called %d times, want 0 (sunset keeps holds)", approval.cleared)
	}
}
