package fixtures

import (
	"fmt"
	"strings"

	"github.com/risk-sentinel/sparc-horizon/internal/view"
)

func pathAPI(parts ...string) string { return "api/" + strings.Join(parts, "/") }

// dateLayout is the contract's date format.
const dateLayout = view.DateLayout

func horizonDate() string { return Horizon.Format(dateLayout) }

// apiResponses builds every golden the mock serves, by running internal/view
// over the OSCAL this run emitted. The tree is the generator's own (frozen in
// #61, and asserted equal to the computed one by cmd/mockserver's tests).
func (g *Generator) apiResponses(tree Tree) (map[string]any, error) {
	docs := map[string][]byte{}
	for p, b := range tree {
		if strings.HasPrefix(p, "oscal/") {
			docs[p] = b
		}
	}
	v, err := view.New(view.LoadFiles(docs), view.Options{})
	if err != nil {
		return nil, err
	}
	if probs := v.Problems(); len(probs) > 0 {
		return nil, fmt.Errorf("the fixtures do not join cleanly: %s: %s (and %d more)", probs[0].Path, probs[0].Detail, len(probs)-1)
	}

	out := map[string]any{}
	personas := g.Personas()
	out[pathAPI("personas.json")] = personas

	for _, p := range personas {
		visible := g.boundariesFor(p)
		out[pathAPI(p.ID, "tree.json")] = g.treeFor(p, visible)
		if next, ok := v.NextAction(p.PartyUUID, p.NodeUUID, Horizon); ok {
			out[pathAPI(p.ID, "next-action.json")] = next
		}
		g.heatResponses(out, v, p, visible)
	}

	out[pathAPI("README.md")] = apiReadme(personas)

	// Chains are not persona-scoped: a chain is reached by requirement UUID,
	// and the mock refuses one outside the caller's tree the same way it
	// refuses a node.
	for _, c := range v.Chains() {
		out[pathAPI("chain", c.Requirement+".json")] = c
	}
	return out, nil
}

// heatResponses adds a persona's heat for every node with rows (its root, and
// each boundary beneath it), and a cell for each drawn column.
func (g *Generator) heatResponses(out map[string]any, v *view.View, p Persona, visible []Boundary) {
	nodes := []string{}
	if p.NodeType == "organization" {
		nodes = append(nodes, p.NodeUUID)
	}
	for _, b := range visible {
		nodes = append(nodes, g.sspUUID(b))
	}
	for _, id := range nodes {
		heat, ok := v.Heat(p.PartyUUID, id, Horizon)
		if !ok {
			continue
		}
		out[pathAPI(p.ID, "heat", id+".json")] = heat
		for _, col := range heat.Columns {
			if cell, ok := v.Cell(p.PartyUUID, id, col, Horizon); ok {
				out[pathAPI(p.ID, "cell", id+"--"+strings.ToLower(col)+".json")] = cell
			}
		}
	}
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
		"Computed by `internal/view`, the same code `go run ./cmd/mockserver` runs live over any",
		"folder of OSCAL (#120). Every response validates against `api/openapi.yaml` in CI — both",
		"that each golden satisfies the schema its path declares, and that the contract declares",
		"nothing the mock cannot answer.",
		"",
		"## These are illustrative shape, not projections",
		"",
		"Cell states come from `project.StateAt` over the observations' expiries and the risks'",
		"blocks-ato. That is the algorithm `docs/05-projection-engine.md` specifies, but this is",
		"**not the projection engine**: there is no ledger, no materialisation, no invalidation along",
		"a node's ancestry, and no ranking model. P2 builds those.",
		"",
		"So: do not cite a number here as an engine result, and do not treat the ordering as the",
		"ranking. What these are good for is shape, volume and the joins between endpoints.",
		"",
		"Every value is derived by **parsing the OSCAL this same run emitted** through the standard",
		"relationships (finding → observation → component, finding → risk → POA&M item), rather than",
		"from the generator's own state. If a response cannot be recomputed from exported documents,",
		"it does not belong in the contract either.",
		"",
		"## Horizon",
		"",
		"These goldens are computed for **"+horizonDate()+"**. The mock computes every response live",
		"from the documents, so it honours the `horizon` query parameter (#120); these files are the",
		"contract's examples and the regression baseline, not what the mock reads.",
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
