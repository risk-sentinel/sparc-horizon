package fixtures

import (
	"testing"

	"github.com/risk-sentinel/sparc-horizon/internal/project"
)

// A column rolls every row, not just the last. Until #117 the column total was
// folded back into Roll as a child with no weight, so a degraded row followed
// by a clean one produced a nominal, collapsed column — the Public Portal's
// heat drew nothing while its next action was a down si-4.
func TestColumnRollsEveryRow(t *testing.T) {
	good := Horizon.AddDate(0, 6, 0)
	pass := func(n int) []assessment {
		as := make([]assessment, n)
		for i := range as {
			as[i] = assessment{family: "SI", nist: true, expires: good}
		}
		return as
	}
	down := append(pass(1), assessment{family: "SI", nist: true, expires: good, failing: true})

	for _, tc := range []struct {
		name string
		rows [][]assessment
	}{
		{"down row first", [][]assessment{down, pass(2)}},
		{"down row last", [][]assessment{pass(2), down}},
		{"down row between", [][]assessment{pass(2), down, pass(2)}},
	} {
		data := make([]rowData, 0, len(tc.rows))
		for _, as := range tc.rows {
			data = append(data, rowData{byFamily: map[string][]assessment{"SI": as}, weight: 1})
		}
		agg := columnAggs(data)["SI"]
		// 1 failing of 5 or 6 assessed: a ratio under 0.85, so degraded
		// wherever the failing row sits.
		if got := project.StateOf(agg); got != project.StateDegraded {
			t.Errorf("%s: column SI is %s (pass %.0f of %.0f), want degraded", tc.name, got, agg.Pass, agg.Total)
		}
	}
}
