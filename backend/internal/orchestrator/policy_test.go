package orchestrator

import (
	"testing"

	"capacitarr/internal/db"
)

func TestPolicyFor_AutoBindings(t *testing.T) {
	p := PolicyFor(db.DiskGroup{
		Mode: db.ModeAuto, ThresholdPct: 85, TargetPct: 75,
	})
	if !p.OK {
		t.Fatal("expected OK")
	}
	if p.EvaluateAt != 85 || p.Target != 75 || p.EscalateAt != 85 {
		t.Errorf("auto bindings evaluateAt=%v target=%v escalateAt=%v", p.EvaluateAt, p.Target, p.EscalateAt)
	}
	if p.HasDurationHold {
		t.Error("auto must not have a duration hold")
	}
	if p.BudgetFromPct() != 75 {
		t.Errorf("BudgetFromPct = %v, want target 75", p.BudgetFromPct())
	}
	if p.ShouldEscalate(90) {
		t.Error("auto should not escalate")
	}
	if !p.ShouldEvaluate(85) || p.ShouldEvaluate(84) {
		t.Error("auto ShouldEvaluate should trip at thresholdPct")
	}
}

func TestPolicyFor_SunsetBindings(t *testing.T) {
	sunsetPct := 70.0
	p := PolicyFor(db.DiskGroup{
		Mode: db.ModeSunset, ThresholdPct: 85, TargetPct: 75, SunsetPct: &sunsetPct,
	})
	if !p.OK {
		t.Fatal("expected OK")
	}
	if p.EvaluateAt != 70 || p.Target != 75 || p.EscalateAt != 85 {
		t.Errorf("sunset bindings evaluateAt=%v target=%v escalateAt=%v", p.EvaluateAt, p.Target, p.EscalateAt)
	}
	if !p.HasDurationHold {
		t.Error("sunset must have a duration hold")
	}
	if p.BudgetFromPct() != 70 {
		t.Errorf("BudgetFromPct = %v, want evaluateAt 70", p.BudgetFromPct())
	}
	if p.ShouldEscalate(84) || !p.ShouldEscalate(85) {
		t.Error("sunset ShouldEscalate should trip at thresholdPct")
	}

	const total int64 = 100
	if got := p.HoldBudgetBytes(90, total); got != 20 {
		t.Errorf("HoldBudgetBytes = %d, want 20", got)
	}
	if got := p.ActionBudgetBytes(90, total); got != 15 {
		t.Errorf("ActionBudgetBytes = %d, want 15", got)
	}
}

func TestPolicyFor_SunsetMisconfigured(t *testing.T) {
	p := PolicyFor(db.DiskGroup{
		Mode: db.ModeSunset, ThresholdPct: 85, TargetPct: 75,
	})
	if p.OK {
		t.Error("expected !OK when sunsetPct is missing")
	}
	if p.ShouldEvaluate(90) || p.ShouldEscalate(90) {
		t.Error("misconfigured sunset must not evaluate or escalate")
	}
}

func TestPolicyFor_DryRunAndApprovalMatchAuto(t *testing.T) {
	auto := PolicyFor(db.DiskGroup{Mode: db.ModeAuto, ThresholdPct: 80, TargetPct: 70})
	for _, mode := range []string{db.ModeDryRun, db.ModeApproval} {
		p := PolicyFor(db.DiskGroup{Mode: mode, ThresholdPct: 80, TargetPct: 70})
		if p.EvaluateAt != auto.EvaluateAt || p.Target != auto.Target || p.EscalateAt != auto.EscalateAt {
			t.Errorf("%s bindings %+v, want same numbers as auto %+v", mode, p, auto)
		}
		if p.HasDurationHold {
			t.Errorf("%s must not have a duration hold", mode)
		}
	}
}
