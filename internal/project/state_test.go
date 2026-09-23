package project

import (
	"testing"
	"time"
)

var (
	horizon = time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	before  = horizon.AddDate(0, 0, -1)
	after   = horizon.AddDate(0, 0, 1)
)

// The three ways to be down are ORed, and they are the whole of it. A control
// that passes today and whose evidence expires before the horizon is down at
// that horizon — which is the difference between projecting and reporting.
func TestStateAtHasThreeWaysDown(t *testing.T) {
	cases := []struct {
		name string
		c    Control
		down bool
	}{
		{"passing, evidence good", Control{Expires: &after}, false},
		{"failing now", Control{Failing: true, Expires: &after}, true},
		{"evidence expired before the horizon", Control{Expires: &before}, true},
		{"evidence expires exactly at the horizon", Control{Expires: &horizon}, true},
		{"open milestone falls before the horizon", Control{Expires: &after, MilestoneOpen: true, Milestone: &before}, true},
		{"closed milestone before the horizon", Control{Expires: &after, MilestoneOpen: false, Milestone: &before}, false},
		{"open milestone after the horizon", Control{Expires: &after, MilestoneOpen: true, Milestone: &after}, false},
		{"no expiry at all", Control{}, false},
	}
	for _, c := range cases {
		if got := StateAt(c.c, horizon).Down; got != c.down {
			t.Errorf("%s: down=%v, want %v", c.name, got, c.down)
		}
	}
}

// "Expires exactly at the horizon" is down, because the rule is
// !Expires.After(t) rather than Before. Evidence that runs out on the day you
// are asking about has run out.
func TestExpiryBoundaryIsInclusive(t *testing.T) {
	if !StateAt(Control{Expires: &horizon}, horizon).Down {
		t.Error("evidence expiring at the horizon must read as down")
	}
	if StateAt(Control{Expires: &after}, horizon).Down {
		t.Error("evidence expiring after the horizon must not")
	}
}

// Blocks is down AND blocks-ato, not blocks-ato alone. A risk that carries the
// prop while the control passes is not blocking anything today.
func TestBlocksRequiresBeingDown(t *testing.T) {
	passing := StateAt(Control{Expires: &after, BlocksATO: true}, horizon)
	if passing.Blocks {
		t.Error("a passing control with blocks-ato is not blocking")
	}
	failing := StateAt(Control{Failing: true, BlocksATO: true}, horizon)
	if !failing.Blocks {
		t.Error("a failing control with blocks-ato must block")
	}
	noProp := StateAt(Control{Failing: true}, horizon)
	if noProp.Blocks {
		t.Error("a failing control without blocks-ato does not block an ATO")
	}
	if !StateAt(Control{Inherited: true, Expires: &after}, horizon).Inherited {
		t.Error("inherited must survive the projection")
	}
}

// A single blocker beneath a node stays visible at every tier above it. A
// rollup that averaged blockers away would let a red boundary disappear into a
// green organization.
func TestRollKeepsBlockersVisible(t *testing.T) {
	kids := []Agg{
		{Total: 10, Pass: 10, Weight: 1},
		{Total: 10, Pass: 9, Weight: 1, Blockers: 1},
		{Total: 10, Pass: 10, Weight: 1},
	}
	got := Roll(kids)
	if got.Blockers != 1 {
		t.Errorf("blockers = %d, want 1 — summed, not averaged", got.Blockers)
	}
	if StateOf(got) != StateBlocks {
		t.Errorf("state = %s; one blocker in 30 controls still blocks", StateOf(got))
	}
}

func TestRollWeightsByImpact(t *testing.T) {
	// The same pass ratio at different impact levels contributes differently.
	high := Agg{Total: 10, Pass: 5, Weight: FIPSWeight("high")}
	low := Agg{Total: 10, Pass: 10, Weight: FIPSWeight("low")}

	got := Roll([]Agg{high, low})
	if got.Total != 40 {
		t.Errorf("total = %v, want 10*3 + 10*1", got.Total)
	}
	if got.Pass != 25 {
		t.Errorf("pass = %v, want 5*3 + 10*1", got.Pass)
	}
	// Unweighted this would be 15/20 = 0.75; weighted it is 25/40 = 0.625.
	// Both are degraded, but the weighting moved it, which is the point.
	if ratio := got.Pass / got.Total; ratio >= 0.75 {
		t.Errorf("ratio = %v; the high-impact child should pull it below the unweighted 0.75", ratio)
	}

	if got.Worsening {
		t.Error("worsening should not appear from nowhere")
	}
	if !Roll([]Agg{{Worsening: true}, {}}).Worsening {
		t.Error("worsening must propagate upward from any child")
	}
}

func TestStateOfThresholds(t *testing.T) {
	cases := []struct {
		name  string
		agg   Agg
		state CellState
	}{
		{"all passing", Agg{Total: 100, Pass: 100}, StateNominal},
		{"exactly at the nominal threshold", Agg{Total: 100, Pass: 95}, StateNominal},
		{"just below nominal", Agg{Total: 100, Pass: 94}, StateWatch},
		{"exactly at the watch threshold", Agg{Total: 100, Pass: 85}, StateWatch},
		{"just below watch", Agg{Total: 100, Pass: 84}, StateDegraded},
		{"nothing passing", Agg{Total: 100, Pass: 0}, StateDegraded},
		{"a blocker beats a perfect ratio", Agg{Total: 100, Pass: 100, Blockers: 1}, StateBlocks},
		{"nothing assessed", Agg{}, StateNominal},
	}
	for _, c := range cases {
		if got := StateOf(c.agg); got != c.state {
			t.Errorf("%s: %s, want %s", c.name, got, c.state)
		}
	}
}

func TestFIPSWeight(t *testing.T) {
	if FIPSWeight("high") <= FIPSWeight("moderate") || FIPSWeight("moderate") <= FIPSWeight("low") {
		t.Error("weights must increase with impact")
	}
	if FIPSWeight("") != 0 || FIPSWeight("catastrophic") != 0 {
		t.Error("an unrecognised impact level weighs nothing rather than guessing")
	}
}
