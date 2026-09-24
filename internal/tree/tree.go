// Package tree builds the federation → organization → boundary → system tree
// from exported OSCAL, with the roles declared at each node.
//
// The tree is a JOIN, not a hierarchy table. The same organization party UUID
// appears in every SSP beneath it, so the shape falls out of the documents
// rather than being stored anywhere — which is what makes the audit test
// (docs/03-data-model.md) possible: every node must be recomputable from
// exported OSCAL alone.
//
// Nodes carry the roles BOUND AT THAT NODE, not the roles effective there.
// api/openapi.yaml calls the field "role bindings", and docs/07-security.md
// says roles "bind to a node and inherit downward" — inheritance is an
// authorization question, evaluated against a request. Pre-expanding it here
// would make the tree lossy about where a role was actually declared, and
// where it was declared is the thing an assessor asks about.
package tree

// NodeType is the tier a node sits in. The values are fixed by
// api/openapi.yaml and are not Horizon's to extend here.
type NodeType string

const (
	Federation   NodeType = "federation"
	Organization NodeType = "organization"
	Boundary     NodeType = "boundary"
	System       NodeType = "system"
)

// Role is a role binding. The contract enumerates exactly these three; a
// role-id in a document that is not one of them is reported rather than
// carried, because emitting it would put a value in the response that the
// frozen schema rejects.
type Role string

const (
	AuthorizingOfficial Role = "authorizing-official"
	SystemOwner         Role = "system-owner"
	ISSO                Role = "information-system-security-officer"
)

func knownRole(id string) (Role, bool) {
	switch Role(id) {
	case AuthorizingOfficial:
		return AuthorizingOfficial, true
	case SystemOwner:
		return SystemOwner, true
	case ISSO:
		return ISSO, true
	}
	return "", false
}

// Node matches the Node schema in api/openapi.yaml, frozen in #61.
// Roles is never nil: the schema requires the property, and a null would fail
// validation where an empty array is the honest answer.
type Node struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	NodeType NodeType `json:"nodeType"`
	Roles    []Role   `json:"roles"`
	Children []*Node  `json:"children,omitempty"`
}

// Finding is something that did not join. It is RETURNED, never logged and
// dropped: P1 task 5 exists because a node silently absent from the tree is a
// node an assessor cannot ask about, and a missing row looks identical to a
// clean one.
type Finding struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	Source string `json:"source"`
}

// Finding kinds, named so callers can assert on them without matching prose.
const (
	FindingUnknownParty      = "unknown-party"
	FindingUnknownRole       = "unknown-role"
	FindingNoOrganization    = "no-organization"
	FindingOrphanInventory   = "orphan-inventory-item"
	FindingDuplicateSystemID = "duplicate-system-id"
	FindingFederationSplit   = "federation-split"
	FindingNoFederation      = "no-federation-party"
)

// Result is the tree and everything that did not fit in it.
type Result struct {
	Root     *Node     `json:"root"`
	Findings []Finding `json:"findings,omitempty"`
}
