package fixtures

import (
	"fmt"
	"sort"
	"strings"

	"github.com/risk-sentinel/sparc-horizon/internal/project"
)

// Building the golden responses, once the OSCAL exists to derive them from.

func pathAPI(parts ...string) string { return "api/" + strings.Join(parts, "/") }

// dateLayout is the one date format every response carries: the OSCAL date
// form, which is also what the namespace props use.
const dateLayout = "2006-01-02"

func horizonDate() string { return Horizon.Format(dateLayout) }

// apiResponses builds every golden the mock serves.
func (g *Generator) apiResponses(tree Tree) (map[string]any, error) {
	assessed := map[string][]assessment{}
	for _, b := range Boundaries {
		a, err := g.assessmentsFor(tree, b)
		if err != nil {
			return nil, err
		}
		assessed[b.Slug] = a
	}

	out := map[string]any{}
	personas := g.Personas()
	out[pathAPI("personas.json")] = personas

	for _, p := range personas {
		visible := g.boundariesFor(p)

		out[pathAPI(p.ID, "tree.json")] = g.treeFor(p, visible)
		out[pathAPI(p.ID, "next-action.json")] = g.nextActionFor(p, visible, assessed)

		// Heat for every node with children: the persona's root, and each
		// boundary beneath it. A system is a leaf and has no rows.
		for _, node := range g.heatNodes(p, visible) {
			heat, err := g.heatFor(node, assessed)
			if err != nil {
				return nil, err
			}
			out[pathAPI(p.ID, "heat", node.id+".json")] = heat

			// Cell drill-downs, for the columns that have something to say.
			for _, col := range heat.Columns {
				detail := g.cellFor(node, col, assessed)
				if len(detail.Controls) == 0 {
					continue
				}
				out[pathAPI(p.ID, "cell", node.id+"--"+strings.ToLower(col)+".json")] = detail
			}
		}
	}

	out[pathAPI("README.md")] = apiReadme(personas)

	// Chains are not persona-scoped: a chain is reached by requirement UUID,
	// and the mock refuses one outside the caller's tree the same way it
	// refuses a node.
	for _, b := range Boundaries {
		for _, a := range assessed[b.Slug] {
			out[pathAPI("chain", a.requirement+".json")] = chainOf(a)
		}
	}
	return out, nil
}

// heatNode is a node the heatmap can be drawn for.
type heatNode struct {
	id       string
	name     string
	nodeType string
	children []Boundary
	systems  []System
	boundary *Boundary
}

func (g *Generator) boundariesFor(p Persona) []Boundary {
	var out []Boundary
	for _, b := range Boundaries {
		switch p.NodeType {
		case "organization":
			if g.orgPartyUUID(b.Org()) == p.NodeUUID {
				out = append(out, b)
			}
		case "boundary":
			if g.sspUUID(b) == p.NodeUUID {
				out = append(out, b)
			}
		}
	}
	return out
}

func (g *Generator) heatNodes(p Persona, visible []Boundary) []heatNode {
	var out []heatNode
	if p.NodeType == "organization" {
		out = append(out, heatNode{id: p.NodeUUID, name: p.Name, nodeType: "organization", children: visible})
	}
	for i := range visible {
		b := visible[i]
		out = append(out, heatNode{id: g.sspUUID(b), name: b.Name, nodeType: "boundary", systems: b.Systems, boundary: &b})
	}
	return out
}

func (g *Generator) treeFor(p Persona, visible []Boundary) APINode {
	root := APINode{ID: p.NodeUUID, Name: p.Name, NodeType: p.NodeType, Roles: []string{p.Role}}
	if p.NodeType == "organization" {
		root.Name = visible[0].Org().Name
		for _, b := range visible {
			root.Children = append(root.Children, g.boundaryNode(b, p.Role))
		}
		return root
	}
	b := visible[0]
	node := g.boundaryNode(b, p.Role)
	node.Roles = []string{p.Role}
	return node
}

func (g *Generator) boundaryNode(b Boundary, role string) APINode {
	node := APINode{ID: g.sspUUID(b), Name: b.Name, NodeType: "boundary", Roles: []string{role}}
	for _, s := range b.Systems {
		node.Children = append(node.Children, APINode{
			ID: g.componentUUID(b, s.Slug), Name: s.Name, NodeType: "system", Roles: []string{role},
		})
	}
	return node
}

// heatFor draws one node's heatmap: rows are its children, columns are
// families. The column order is blockers descending then pass ratio ascending,
// and nominal columns collapse — management by exception, so green is quiet.
//
// A row carries a cell only for the columns that survived collapsing, in
// column order. docs/04-api.md's own example has five columns and two cells,
// and says to keep cell payloads small; details come from the cell and chain
// endpoints.
// heatFor draws one node's heatmap: rows are its children, columns are
// families. The column order is blockers descending then pass ratio ascending,
// and nominal columns collapse — management by exception, so green is quiet.
func (g *Generator) heatFor(node heatNode, assessed map[string][]assessment) (APIHeat, error) {
	data := g.rowsOf(node, assessed)

	// Column aggregates first, because the order and the collapsing decide
	// which cells a row emits at all.
	colAgg := map[string]project.Agg{}
	for _, r := range data {
		for fam, as := range r.byFamily {
			agg, _ := aggregate(as, r.weight)
			colAgg[fam] = project.Roll([]project.Agg{colAgg[fam], agg})
		}
	}
	columns, collapsed := orderColumns(colAgg)

	rows := make([]APIRow, 0, len(data))
	for _, r := range data {
		rows = append(rows, r.render(columns))
	}

	return APIHeat{
		Node:      node.id,
		Horizon:   horizonDate(),
		Axis:      "800-53",
		Columns:   columns,
		Collapsed: collapsed,
		Rows:      rows,
	}, nil
}

// rowData is one heat row before the columns are known.
type rowData struct {
	id, name string
	byFamily map[string][]assessment
	weight   float64
}

// render emits cells for the columns that survived collapsing, in column
// order. A family the row has nothing in is omitted rather than drawn as
// passing: they are not the same claim, and the HUD draws a gap.
func (r rowData) render(columns []string) APIRow {
	row := APIRow{ID: r.id, Name: r.name, Cells: []APICell{}}
	for _, col := range columns {
		as, ok := r.byFamily[col]
		if !ok {
			continue
		}
		_, cell := aggregate(as, r.weight)
		row.Cells = append(row.Cells, cell)
	}
	return row
}

// rowsOf gathers a node's children as rows: an organization's rows are its
// boundaries, a boundary's rows are its systems.
func (g *Generator) rowsOf(node heatNode, assessed map[string][]assessment) []rowData {
	collect := func(id, name string, as []assessment, weight float64) rowData {
		byFamily := map[string][]assessment{}
		for _, a := range onAxis(as) {
			byFamily[a.family] = append(byFamily[a.family], a)
		}
		return rowData{id: id, name: name, byFamily: byFamily, weight: weight}
	}

	var data []rowData
	if node.nodeType == "organization" {
		for _, b := range node.children {
			data = append(data, collect(g.sspUUID(b), b.Name, assessed[b.Slug], project.FIPSWeight(b.FIPS)))
		}
		return data
	}

	b := *node.boundary
	byComponent := map[string][]assessment{}
	for _, a := range assessed[b.Slug] {
		byComponent[a.component] = append(byComponent[a.component], a)
	}
	for _, s := range b.Systems {
		id := g.componentUUID(b, s.Slug)
		data = append(data, collect(id, s.Name, byComponent[id], project.FIPSWeight(b.FIPS)))
	}
	return data
}

// onAxis keeps the controls the 800-53 column axis can actually render.
//
// Security Hub identifiers reach a boundary through inherited AWS component
// definitions and are keyed to that authority, not to NIST. Putting `ACM` and
// `S3` beside `AC` and `CP` would claim a crosswalk this repository does not
// own and has not applied — the mapping is SPARC's (`sparc#1103`, and X-8 in
// the implementation plan). They are in the fixtures precisely so the
// distinction is exercised; they are off this axis until the crosswalk is.
func onAxis(as []assessment) []assessment {
	out := make([]assessment, 0, len(as))
	for _, a := range as {
		if a.nist {
			out = append(out, a)
		}
	}
	return out
}

func aggregate(as []assessment, weight float64) (project.Agg, APICell) {
	agg := project.Agg{Weight: weight}
	cell := APICell{}
	for _, a := range as {
		st := a.state()
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
		// Worsening: evidence that expires inside the look-ahead window, so
		// the cell is on its way down even where it passes today.
		if !st.Down && a.expires.Before(Horizon.AddDate(0, 0, 30)) {
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

// orderColumns ranks by blockers descending then pass ratio ascending, and
// collapses the nominal ones — management by exception, so green is quiet.
func orderColumns(colAgg map[string]project.Agg) (columns, collapsed []string) {
	keys := sortedKeys(colAgg)
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

func (g *Generator) cellFor(node heatNode, col string, assessed map[string][]assessment) APICellDetail {
	detail := APICellDetail{Node: node.id, Col: col, Horizon: horizonDate()}

	var pool []assessment
	if node.nodeType == "organization" {
		for _, b := range node.children {
			pool = append(pool, assessed[b.Slug]...)
		}
	} else {
		pool = assessed[node.boundary.Slug]
	}

	for _, a := range onAxis(pool) {
		if a.family != col {
			continue
		}
		st := a.state()
		if !st.Down {
			continue
		}
		detail.Controls = append(detail.Controls, APIControl{
			ControlID: a.controlID,
			Component: a.component,
			State:     string(project.StateOf(project.Agg{Total: 1, Blockers: boolInt(st.Blocks)})),
			Reason:    reasonFor(a),
			Expires:   a.expires.Format(dateLayout),
			BlocksATO: a.blocksATO,
			Chain:     a.requirement,
		})
	}
	// Blocking first, then by control id, so the ranking is reproducible.
	sort.SliceStable(detail.Controls, func(i, j int) bool {
		if detail.Controls[i].BlocksATO != detail.Controls[j].BlocksATO {
			return detail.Controls[i].BlocksATO
		}
		return detail.Controls[i].ControlID < detail.Controls[j].ControlID
	})
	return detail
}

// reasonFor states the consequence with the item, which is the HUD's rule.
func reasonFor(a assessment) string {
	switch {
	case a.failing && a.blocksATO:
		return "Not satisfied, and the open risk blocks the authorization."
	case a.failing:
		return "Not satisfied at the assessment."
	case !a.expires.After(Horizon):
		return fmt.Sprintf("Evidence expires %s, before the %s horizon.", a.expires.Format(dateLayout), horizonDate())
	default:
		return "Down at the horizon."
	}
}

func chainOf(a assessment) APIChain {
	return APIChain{
		Requirement: a.requirement,
		ControlID:   a.controlID,
		Component:   a.component,
		Resource:    a.resource,
		Observation: a.observation,
		Expires:     a.expires.Format(dateLayout),
		Finding:     a.finding,
		Risk:        a.risk,
		BlocksATO:   a.blocksATO,
		PoamItem:    a.poamItem,
	}
}

// nextActionFor is the single focal point a lens offers: one next best action,
// with the consequence stated beside it.
func (g *Generator) nextActionFor(p Persona, visible []Boundary, assessed map[string][]assessment) APINextAction {
	var best *assessment
	for _, b := range visible {
		for i, a := range assessed[b.Slug] {
			st := a.state()
			if !st.Down {
				continue
			}
			if best == nil || (st.Blocks && !best.blocksATO) ||
				(st.Blocks == best.blocksATO && a.expires.Before(best.expires)) {
				best = &assessed[b.Slug][i]
			}
		}
	}
	if best == nil {
		return APINextAction{
			Node: p.NodeUUID, Title: "Nothing is down at this horizon.",
			Consequence: "No control in view fails, expires or has an open milestone before " + horizonDate() + ".",
			Horizon:     horizonDate(),
		}
	}
	consequence := "The control is down at the horizon."
	if best.blocksATO {
		consequence = "An open risk on this control blocks the authorization for " + best.boundary.Name + "."
	}
	return APINextAction{
		Node:        p.NodeUUID,
		Title:       "Resolve " + best.controlID + " on " + best.boundary.Name,
		Consequence: consequence,
		ControlID:   best.controlID,
		Chain:       best.requirement,
		Horizon:     horizonDate(),
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// apiReadme is generated with the goldens so it cannot describe a version of
// them that no longer exists.
func apiReadme(personas []Persona) string {
	var b strings.Builder
	w := func(lines ...string) {
		for _, l := range lines {
			b.WriteString(l)
			b.WriteString("\n")
		}
	}
	w(
		"# API v0 golden responses",
		"",
		"**Generated. Do not edit by hand.** Written by `internal/fixtures` alongside the OSCAL,",
		"and rewritten wholesale by `go run ./cmd/genfixtures`.",
		"",
		"Served by `go run ./cmd/mockserver`, which P3 builds the HUD against and P1 shapes its",
		"client against. Every response validates against `api/openapi.yaml` in CI — both that each",
		"golden satisfies the schema its path declares, and that the contract declares nothing the",
		"mock cannot answer.",
		"",
		"## These are illustrative shape, not projections",
		"",
		"Cell states come from `project.StateAt` over the fixtures' own observation expiries and",
		"blocking risks. That is the algorithm `docs/05-projection-engine.md` specifies, but this is",
		"**not the projection engine**: there is no ledger, no materialisation, no invalidation along",
		"a node's ancestry, and no ranking model. P2 builds those.",
		"",
		"So: do not cite a number here as an engine result, and do not treat the ordering as the",
		"ranking. What these are good for is shape, volume and the joins between endpoints.",
		"",
		"Every value is derived by **parsing the OSCAL this same run emitted**, rather than from the",
		"generator's own state. That is the cheap version of the audit claim — if a response cannot",
		"be recomputed from exported documents, it does not belong in the contract either.",
		"",
		"## Horizon",
		"",
		"Computed once, for **"+horizonDate()+"**. The `horizon` query parameter is",
		"accepted and ignored by the mock; a real service projects to the date it is asked for.",
		"",
		"## Personas",
		"",
		"Roles bind to a node and inherit downward, so each persona sees a **different tree** rather",
		"than the same tree with parts greyed out. Select one with the `X-Horizon-Persona` header.",
		"",
		"| Persona | Role | Sees |",
		"|---|---|---|",
	)
	for _, p := range personas {
		w("| `" + p.ID + "` | " + p.Role + " | " + p.About + " |")
	}
	w(
		"",
		"**A node outside the caller's tree returns 404, not 403** — identical to a node that does",
		"not exist. The mock enforces that rather than describing it, because a client written",
		"against a lenient mock is a client that breaks on the real service.",
		"",
		"## The 800-53 axis",
		"",
		"Heat and cell responses carry NIST families only. The fixtures also contain AWS Security Hub",
		"controls, reaching a boundary through an inherited component definition — they are **off this",
		"axis** until SPARC's crosswalk maps them (`sparc#1103`). Putting `ACM` beside `AC` would claim",
		"a mapping this repository does not own.",
	)
	return b.String()
}
