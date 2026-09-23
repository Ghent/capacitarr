package orchestrator

// RunAccumulator collects per-cycle metrics across multiple disk group
// evaluations within a single engine run. Each disk group gets its own
// GroupAccumulator. Not shared across goroutines — the poller runs
// single-threaded.
type RunAccumulator struct {
	Groups map[uint]*GroupAccumulator
}

// NewRunAccumulator creates a RunAccumulator with an initialized map.
func NewRunAccumulator() *RunAccumulator {
	return &RunAccumulator{Groups: make(map[uint]*GroupAccumulator)}
}

// GetOrCreate returns the accumulator for a disk group, creating it if needed.
func (a *RunAccumulator) GetOrCreate(groupID uint, mountPath, mode string) *GroupAccumulator {
	if ga, ok := a.Groups[groupID]; ok {
		return ga
	}
	ga := &GroupAccumulator{MountPath: mountPath, Mode: mode}
	a.Groups[groupID] = ga
	return ga
}

// Totals returns aggregate counts across all groups for engine stats.
func (a *RunAccumulator) Totals() (evaluated, candidates, protected, collections int64, freedBytes int64) {
	for _, ga := range a.Groups {
		evaluated += ga.Evaluated
		candidates += ga.Candidates
		protected += ga.Protected
		collections += ga.Collections
		freedBytes += ga.FreedBytes
	}
	return
}

// GroupAccumulator collects per-group metrics for a single disk group evaluation.
type GroupAccumulator struct {
	MountPath     string
	Mode          string
	Evaluated     int64
	Candidates    int64
	Protected     int64
	FreedBytes    int64
	Collections   int64
	DiskUsagePct  float64
	DiskThreshold float64
	DiskTargetPct float64
	// Sunset-mode counters (zero for other modes)
	SunsetQueued int
	// Items not dispatched because the in-memory deletion queue was full.
	QueueFullSkipped int
}
