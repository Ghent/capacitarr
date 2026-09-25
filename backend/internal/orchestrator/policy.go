package orchestrator

import "capacitarr/internal/db"

// DiskGroupPolicy is the §2–§3 binding of a disk-group mode onto the
// shared machine. Storage stays threshold_pct / sunset_pct / target_pct;
// these names are the model.
type DiskGroupPolicy struct {
	Mode            string
	EvaluateAt      float64
	Target          float64
	EscalateAt      float64
	OK              bool
	HasDurationHold bool
}

// PolicyFor binds a disk group's stored thresholds to the shared-machine
// numbers. OK is false when sunset is missing sunsetPct.
func PolicyFor(group db.DiskGroup) DiskGroupPolicy {
	if group.Mode == db.ModeSunset {
		if group.SunsetPct == nil {
			return DiskGroupPolicy{
				Mode:            group.Mode,
				Target:          group.TargetPct,
				EscalateAt:      group.ThresholdPct,
				HasDurationHold: true,
			}
		}
		return DiskGroupPolicy{
			Mode:            group.Mode,
			EvaluateAt:      *group.SunsetPct,
			Target:          group.TargetPct,
			EscalateAt:      group.ThresholdPct,
			OK:              true,
			HasDurationHold: true,
		}
	}
	return DiskGroupPolicy{
		Mode:       group.Mode,
		EvaluateAt: group.ThresholdPct,
		Target:     group.TargetPct,
		EscalateAt: group.ThresholdPct,
		OK:         true,
	}
}

// BudgetFromPct is the percent used to size the Admit budget: hold budget
// (used − evaluateAt) for duration-hold presets, action budget (used − target)
// otherwise.
func (p DiskGroupPolicy) BudgetFromPct() float64 {
	if p.HasDurationHold {
		return p.EvaluateAt
	}
	return p.Target
}

// ShouldEvaluate reports used >= evaluateAt.
func (p DiskGroupPolicy) ShouldEvaluate(usedPct float64) bool {
	return p.OK && usedPct >= p.EvaluateAt
}

// ShouldEscalate reports a duration-hold preset at or above escalateAt.
func (p DiskGroupPolicy) ShouldEscalate(usedPct float64) bool {
	return p.OK && p.HasDurationHold && usedPct >= p.EscalateAt
}

// ActionBudgetBytes is used − target, in bytes of effectiveTotal.
func (p DiskGroupPolicy) ActionBudgetBytes(usedPct float64, effectiveTotal int64) int64 {
	return int64((usedPct - p.Target) / 100.0 * float64(effectiveTotal))
}

// HoldBudgetBytes is used − evaluateAt, in bytes of effectiveTotal.
func (p DiskGroupPolicy) HoldBudgetBytes(usedPct float64, effectiveTotal int64) int64 {
	return int64((usedPct - p.EvaluateAt) / 100.0 * float64(effectiveTotal))
}
