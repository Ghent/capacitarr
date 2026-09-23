package poller

import (
	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
)

// evaluateDiskGroup is a thin wrapper so existing poller tests keep calling
// the unexported production path. The product loop lives on Orchestrator.
func (p *Poller) evaluateDiskGroup(acc *RunAccumulator, group db.DiskGroup, allItems []integrations.MediaItem, registry *integrations.IntegrationRegistry, runStatsID uint, prefs db.PreferenceSet, weights map[string]int, rules []db.CustomRule, evalCtx *engine.EvaluationContext) int {
	return p.orch.EvaluateDiskGroup(acc, group, allItems, registry, runStatsID, prefs, weights, rules, evalCtx)
}
