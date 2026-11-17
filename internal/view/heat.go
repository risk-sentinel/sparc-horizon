package view

import (
	"fmt"
	"sort"
	"time"

	"github.com/risk-sentinel/sparc-horizon/internal/project"
	"github.com/risk-sentinel/sparc-horizon/internal/tree"
)

// worseningWindow is how far ahead an expiry marks a passing cell as on its
// way down.
const worseningWindow = 30

// rowData is one heat row before the columns are known.
type rowData struct {
	id, name string
	byFamily map[string][]assessment
	weight   float64
}

// node resolves a node a party may see and that has rows: an organization or
// a boundary.
func (v *View) node(party, id string) (*tree.Node, []*boundary, bool) {
	n, ok := v.nodes[id]
	if !ok || !v.visible(party, id) {
		return nil, nil, false
	}
	bs := v.boundariesUnder(n)
	if len(bs) == 0 {
		return nil, nil, false
	}
	return n, bs, true
}

// Heat draws a node's heatmap at a horizon: rows are its children, columns are
// 800-53 families. Columns order by blockers descending, then pass ratio
// ascending, and nominal ones collapse: management by exception.
func (v *View) Heat(party, id string, at time.Time) (Heat, bool) {
	n, bs, ok := v.node(party, id)
	if !ok {
		return Heat{}, false
	}
	data := v.rowsOf(n, bs)
	columns, collapsed := orderColumns(columnAggs(data, at))
	rows := make([]Row, 0, len(data))
	for _, r := range data {
		rows = append(rows, r.render(columns, at))
	}
	return Heat{
		Node: id, Horizon: at.Format(DateLayout), Axis: "800-53",
		Columns: columns, Collapsed: collapsed, Rows: rows,
	}, true
}

// rowsOf gathers a node's children as rows, in tree order: an organization's
// rows are its boundaries, a boundary's rows are its systems.
func (v *View) rowsOf(n *tree.Node, bs []*boundary) []rowData {
	collect := func(id, name string, as []assessment, weight float64) rowData {
		byFamily := map[string][]assessment{}
		for _, a := range as {
			if a.nist {
				byFamily[a.family] = append(byFamily[a.family], a)
			}
		}
		return rowData{id: id, name: name, byFamily: byFamily, weight: weight}
	}

	var data []rowData
	if n.NodeType == tree.Organization {
		for _, b := range bs {
			data = append(data, collect(b.id(), b.name, v.assessed[b.id()], b.weight()))
		}
		return data
	}
	b := bs[0]
	byComponent := map[string][]assessment{}
	for _, a := range v.assessed[b.id()] {
		byComponent[a.component] = append(byComponent[a.component], a)
	}
	for _, s := range n.Children {
		data = append(data, collect(s.ID, s.Name, byComponent[s.ID], b.weight()))
	}
	return data
}

// columnAggs rolls every row into each family's column in one Roll call.
// Roll returns an Agg with no weight of its own, so folding a running total
// back in as a child would multiply it away (#117).
func columnAggs(data []rowData, at time.Time) map[string]project.Agg {
	kids := map[string][]project.Agg{}
	for _, r := range data {
		for fam, as := range r.byFamily {
			agg, _ := aggregate(as, r.weight, at)
			kids[fam] = append(kids[fam], agg)
		}
	}
	out := make(map[string]project.Agg, len(kids))
	for fam, k := range kids {
		out[fam] = project.Roll(k)
	}
	return out
}

// render emits cells for the drawn columns, in column order. A family the row
// has nothing in is omitted rather than drawn as passing: they are not the
// same claim, and the HUD draws a gap.
func (r rowData) render(columns []string, at time.Time) Row {
	row := Row{ID: r.id, Name: r.name, Cells: []Cell{}}
	for _, col := range columns {
		as, ok := r.byFamily[col]
		if !ok {
			continue
		}
		_, cell := aggregate(as, r.weight, at)
		row.Cells = append(row.Cells, cell)
	}
	return row
}

func aggregate(as []assessment, weight float64, at time.Time) (project.Agg, Cell) {
	agg := project.Agg{Weight: weight}
	cell := Cell{}
	soon := at.AddDate(0, 0, worseningWindow)
	for _, a := range as {
		st := a.state(at)
		agg.Total++
		if !st.Down {
			agg.Pass++
		}
		if st.Blocks {
			agg.Blockers++
		}
		if st.Inherited {
			cell.Inherited = true
		}
		if !st.Down && a.expires != nil && a.expires.Before(soon) {
			cell.Worsening = true
		}
	}
	if len(as) > 0 {
		cell.Col = as[0].family
	}
	cell.Blockers = agg.Blockers
	cell.State = string(project.StateOf(agg))
	return agg, cell
}

func orderColumns(colAgg map[string]project.Agg) (columns, collapsed []string) {
	keys := make([]string, 0, len(colAgg))
	for k := range colAgg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sort.SliceStable(keys, func(i, j int) bool {
		a, b := colAgg[keys[i]], colAgg[keys[j]]
		if a.Blockers != b.Blockers {
			return a.Blockers > b.Blockers
		}
		return ratio(a) < ratio(b)
	})
	columns, collapsed = []string{}, []string{}
	for _, k := range keys {
		if project.StateOf(colAgg[k]) == project.StateNominal {
			collapsed = append(collapsed, k)
			continue
		}
		columns = append(columns, k)
	}
	return columns, collapsed
}

func ratio(a project.Agg) float64 {
	if a.Total == 0 {
		return 1
	}
	return a.Pass / a.Total
}

// Cell is the drill-down for one drawn column: the controls down in it,
// blocking first, then by control id and component.
func (v *View) Cell(party, id, col string, at time.Time) (CellDetail, bool) {
	heat, ok := v.Heat(party, id, at)
	if !ok || !contains(heat.Columns, col) {
		return CellDetail{}, false
	}
	_, bs, _ := v.node(party, id)
	detail := CellDetail{Node: id, Col: col, Horizon: at.Format(DateLayout)}
	for _, b := range bs {
		detail.Controls = append(detail.Controls, downIn(v.assessed[b.id()], col, at)...)
	}
	if len(detail.Controls) == 0 {
		return CellDetail{}, false
	}
	sortControls(detail.Controls)
	return detail, true
}

// downIn is the on-axis controls in a family that are down at the horizon.
func downIn(as []assessment, col string, at time.Time) []Control {
	var out []Control
	for _, a := range as {
		if !a.nist || a.family != col {
			continue
		}
		st := a.state(at)
		if !st.Down {
			continue
		}
		c := Control{
			ControlID: a.controlID,
			Component: a.component,
			State:     string(project.StateOf(project.Agg{Total: 1, Blockers: boolInt(st.Blocks)})),
			Reason:    reasonFor(a, at),
			BlocksATO: a.blocksATO,
			Chain:     a.requirement,
		}
		if a.expires != nil {
			c.Expires = a.expires.Format(DateLayout)
		}
		out = append(out, c)
	}
	return out
}

func sortControls(cs []Control) {
	sort.SliceStable(cs, func(i, j int) bool {
		x, y := cs[i], cs[j]
		if x.BlocksATO != y.BlocksATO {
			return x.BlocksATO
		}
		if x.ControlID != y.ControlID {
			return x.ControlID < y.ControlID
		}
		return x.Component < y.Component
	})
}

// reasonFor states the consequence with the item, which is the HUD's rule.
func reasonFor(a assessment, at time.Time) string {
	switch {
	case a.failing && a.blocksATO:
		return "Not satisfied, and the open risk blocks the authorization."
	case a.failing:
		return "Not satisfied at the assessment."
	case a.expires != nil && !a.expires.After(at):
		return fmt.Sprintf("Evidence expires %s, before the %s horizon.", a.expires.Format(DateLayout), at.Format(DateLayout))
	default:
		return "Down at the horizon."
	}
}

// NextAction is the single focal point under a node: a blocking control
// first, then the one whose evidence runs out soonest.
func (v *View) NextAction(party, id string, at time.Time) (NextAction, bool) {
	_, bs, ok := v.node(party, id)
	if !ok {
		return NextAction{}, false
	}
	var best *assessment
	for _, b := range bs {
		as := v.assessed[b.id()]
		for i := range as {
			a := &as[i]
			st := a.state(at)
			if !st.Down {
				continue
			}
			if best == nil || (st.Blocks && !best.blocksATO) ||
				(st.Blocks == best.blocksATO && earlier(a.expires, best.expires)) {
				best = a
			}
		}
	}
	horizon := at.Format(DateLayout)
	if best == nil {
		return NextAction{
			Node: id, Title: "Nothing is down at this horizon.",
			Consequence: "No control in view fails, expires or has an open milestone before " + horizon + ".",
			Horizon:     horizon,
		}, true
	}
	consequence := "The control is down at the horizon."
	if best.blocksATO {
		consequence = "An open risk on this control blocks the authorization for " + best.boundary.name + "."
	}
	return NextAction{
		Node: id, Title: "Resolve " + best.controlID + " on " + best.boundary.name,
		Consequence: consequence, ControlID: best.controlID, Chain: best.requirement, Horizon: horizon,
	}, true
}

// earlier orders expiries with "never expires" last.
func earlier(a, b *time.Time) bool {
	switch {
	case a == nil:
		return false
	case b == nil:
		return true
	}
	return a.Before(*b)
}

func contains(xs []string, x string) bool {
	for _, have := range xs {
		if have == x {
			return true
		}
	}
	return false
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
