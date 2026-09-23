package orchestrator

import (
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

type stubApproval struct{}

func (s *stubApproval) ClearQueueForDiskGroup(uint) (int, error) { return 0, nil }
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
