package view

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/risk-sentinel/sparc-horizon/internal/tree"
)

var roleShort = map[tree.Role]string{
	tree.AuthorizingOfficial: "ao",
	tree.SystemOwner:         "so",
	tree.ISSO:                "iso",
}

var tierRank = map[tree.NodeType]int{
	tree.Federation: 0, tree.Organization: 1, tree.Boundary: 2, tree.System: 3,
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Personas are derived from the documents when no personas file is supplied:
// one per party that holds a contract role, at the highest node it holds one.
// A role a component holds is not a persona; a component is not a caller.
//
// The documents decide who sees what here, exactly as the fixtures do. That is
// acceptable only because the persona header is not authentication (#120).
// Real ingest must not let a document grant a role (TM-1).
func (v *View) Personas() []Persona {
	names := v.partyNames()
	best := highestBindings(v.tree.Bindings, names)

	out := make([]Persona, 0, len(best))
	taken := map[string]bool{}
	for party, bd := range best {
		name := names[party]
		if name == "" {
			name = party
		}
		base := roleShort[bd.Role] + "-" + strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
		id := base
		for n := 2; taken[id]; n++ {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		taken[id] = true
		out = append(out, Persona{
			ID: id, Role: string(bd.Role), PartyUUID: party,
			NodeUUID: bd.NodeID, NodeType: string(bd.NodeType), Name: name,
		})
	}
	sortPersonas(out)
	return out
}

func (v *View) partyNames() map[string]string {
	names := map[string]string{}
	for _, b := range v.links.boundaries {
		for _, p := range deref(b.plan.Metadata.Parties) {
			names[p.UUID] = p.Name
		}
	}
	return names
}

// highestBindings is each party's binding at the highest tier.
func highestBindings(bindings []tree.Binding, names map[string]string) map[string]tree.Binding {
	best := map[string]tree.Binding{}
	for _, bd := range bindings {
		if _, isParty := names[bd.PartyUUID]; !isParty {
			continue
		}
		if have, ok := best[bd.PartyUUID]; !ok || tierRank[bd.NodeType] < tierRank[have.NodeType] {
			best[bd.PartyUUID] = bd
		}
	}
	return best
}

func sortPersonas(out []Persona) {
	sort.Slice(out, func(i, j int) bool {
		ri, rj := tierRank[tree.NodeType(out[i].NodeType)], tierRank[tree.NodeType(out[j].NodeType)]
		if ri != rj {
			return ri < rj
		}
		return out[i].ID < out[j].ID
	})
}
