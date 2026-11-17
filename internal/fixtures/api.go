package fixtures

// The API v0 mock's golden responses, computed by internal/view from the OSCAL
// this same run emitted: the same code the mock runs live over any folder of
// documents (#120). So the goldens are what a reader that is not this
// generator gets from these documents, and every number recomputes from
// exported OSCAL alone.
//
// # These are illustrative shape, not projections
//
// Cell state comes from project.StateAt over the observations' expiries and
// the risks' blocks-ato. That is the documented algorithm, but it is not the
// projection engine: there is no ledger, no materialisation, no invalidation
// and no ranking model. P2 builds those.

// Horizon is the date the golden responses project to. Fixed, like everything
// else in this tree: a mock whose answers move with the clock is a mock nobody
// can write a test against.
var Horizon = days(30)

// Persona is a caller the mock can serve. Roles bind to a node and inherit
// downward, so each persona sees a different tree rather than the same tree
// with parts greyed out.
type Persona struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	PartyUUID string `json:"party-uuid"`
	NodeUUID  string `json:"node-uuid"`
	NodeType  string `json:"node-type"`
	Name      string `json:"name"`
	About     string `json:"about"`
}

// APINode is a node in the tree the caller can see.
type APINode struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	NodeType string    `json:"nodeType"`
	Roles    []string  `json:"roles"`
	Children []APINode `json:"children,omitempty"`
}

// Personas are the callers the mock serves, matching P1's exit criteria: an AO
// at an organization, and an SO and an ISO at a boundary.
func (g *Generator) Personas() []Persona {
	org := Orgs[0]
	boundary, other := Boundaries[0], Boundaries[2]
	return []Persona{
		{
			ID: "ao-" + org.Slug, Role: roleAO,
			PartyUUID: g.rolePartyUUID(boundary, roleAO),
			NodeUUID:  g.orgPartyUUID(org), NodeType: "organization", Name: org.Name + " Authorizing Official",
			About: "Sees the organization, its boundaries and their systems. Roles inherit downward.",
		},
		{
			ID: "so-" + boundary.Slug, Role: roleSO,
			PartyUUID: g.rolePartyUUID(boundary, roleSO),
			NodeUUID:  g.sspUUID(boundary), NodeType: "boundary", Name: boundary.Name + " System Owner",
			About: "Sees one boundary and its systems. Everything above it is 404, not 403.",
		},
		{
			ID: "iso-" + other.Slug, Role: roleISO,
			PartyUUID: g.rolePartyUUID(other, roleISO),
			NodeUUID:  g.sspUUID(other), NodeType: "boundary", Name: other.Name + " Information System Security Officer",
			About: "A different organization entirely, to show that two personas do not overlap.",
		},
	}
}
