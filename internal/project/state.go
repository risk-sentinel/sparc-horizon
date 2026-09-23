// Package project holds the projection primitives: what state a control is in
// on a given date, and how child states roll up into a parent's cell.
//
// It is seeded here, in P0, with exactly the two functions
// docs/05-projection-engine.md specifies as source, because the API mock's
// golden responses need them (#61). Writing a private copy inside the
// generator would put two implementations of one documented algorithm in this
// repository, which is the divergence the key grammar exists to prevent, one
// layer up.
//
// # What belongs here, and what does not
//
// In scope: StateAt, Roll, and the cell-state thresholds.
//
// P2's, and deliberately absent: the append-only ledger, materialisation per
// node and horizon bucket, invalidation along a node's ancestry, next-best-
// action ranking, and the recompute audit. The temptation to keep adding here
// is the reason this paragraph exists.
//
// Everything in a cell comes from OSCAL fields, so any cell recomputes from
// the exported documents alone. That recomputation is the audit test.
package project

import "time"

// Control is the projected input for one control on one component: the OSCAL
// fields that decide whether it is down at a date.
type Control struct {
	// Failing is a finding that did not satisfy the control objective.
	Failing bool
	// Expires is the observation's native expiry. It drives every countdown
	// in the HUD; nothing in the namespace props duplicates it.
	Expires *time.Time
	// MilestoneOpen and Milestone are the POA&M milestone and whether it is
	// still open.
	MilestoneOpen bool
	Milestone     *time.Time
	// BlocksATO is the `blocks-ato` prop on an open risk.
	BlocksATO bool
	// Inherited marks a control implemented through an `inherited` statement.
	Inherited bool
}

// State is what a control reads as on a date.
type State struct {
	Down      bool
	Blocks    bool
	Inherited bool
}

// StateAt answers "what is the state of this control on date t".
//
// Three ways to be down, and they are ORed rather than ranked: the control is
// failing now, its evidence has expired by t, or a POA&M milestone falls due
// before t and is still open. Projection is the point — a control that passes
// today and whose evidence expires next week is down at a horizon past that
// week.
func StateAt(c Control, t time.Time) State {
	down := c.Failing ||
		(c.Expires != nil && !c.Expires.After(t)) ||
		(c.MilestoneOpen && c.Milestone != nil && !c.Milestone.After(t))

	return State{Down: down, Blocks: down && c.BlocksATO, Inherited: c.Inherited}
}

// Agg is a weighted tally of child states, which is what a cell renders.
type Agg struct {
	Total     float64
	Pass      float64
	Weight    float64
	Blockers  int
	Worsening bool
}

// Roll combines children into a parent.
//
// Totals and passes are weighted by FIPS 199 impact; blockers are summed
// rather than averaged, so a single blocking risk anywhere beneath a node
// stays visible at every tier above it. That is deliberate: a rollup that
// averaged blockers away would let a red boundary disappear into a green
// organization.
func Roll(kids []Agg) (a Agg) {
	for _, k := range kids {
		a.Total += k.Total * k.Weight
		a.Pass += k.Pass * k.Weight
		a.Blockers += k.Blockers
		a.Worsening = a.Worsening || k.Worsening
	}
	return a
}

// CellState is the four-value vocabulary every tier renders with.
type CellState string

const (
	StateNominal  CellState = "nominal"
	StateWatch    CellState = "watch"
	StateDegraded CellState = "degraded"
	StateBlocks   CellState = "blocks"
)

// Cell-state thresholds, from docs/05-projection-engine.md. They also appear
// in api/openapi.yaml, docs/04-api.md and the demos' cls(); changing one means
// changing all of them.
const (
	NominalRatio = 0.95
	WatchRatio   = 0.85
)

// StateOf reduces an aggregate to the cell state it renders as.
//
// Blockers dominate the ratio: a cell with any ATO blocker is `blocks`
// however well the rest of it is doing, because the consequence is not
// proportional to the count.
func StateOf(a Agg) CellState {
	if a.Blockers > 0 {
		return StateBlocks
	}
	if a.Total == 0 {
		// Nothing assessed is not the same as everything passing, but the
		// cell has nothing to render either way. The HUD collapses these;
		// what it must not do is show them as failing.
		return StateNominal
	}
	switch ratio := a.Pass / a.Total; {
	case ratio >= NominalRatio:
		return StateNominal
	case ratio >= WatchRatio:
		return StateWatch
	default:
		return StateDegraded
	}
}

// FIPSWeight is the rollup weight for an impact level.
//
// **The design does not fix these values.** docs/05-projection-engine.md says
// totals are weighted by FIPS 199 and gives no numbers, and the ranking
// weights it does discuss are explicitly configuration. These are a
// documented default so the mock's responses are reproducible, not a decision:
// carried as an open item in docs/10-risks-decisions.md for P2 to settle with
// users, because the weighting changes which boundary a person is told to look
// at first.
func FIPSWeight(fips string) float64 {
	switch fips {
	case "high":
		return 3
	case "moderate":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
