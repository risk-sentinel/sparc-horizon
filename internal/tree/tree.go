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

import "github.com/google/uuid"

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
	// FindingDuplicateBoundary: two SSPs name the same SPARC boundary uuid
	// (`parent-uuid` under `node-type: system`). SPARC holds one SSP per
	// boundary, so this is two exports of one boundary, or a copy. Each keeps
	// its own document uuid as its node id rather than sharing one node.
	FindingDuplicateBoundary = "duplicate-boundary"
	// FindingImplicitFederation is informational: an SSP named one organization
	// and no federation above it, so the tree placed it under LocalFederation.
	FindingImplicitFederation = "implicit-federation"
)

// LocalFederation is the root an organization is placed under when its SSP
// names no federation, which is the common case outside the SPARC estate: one
// organization party, a member of nothing. Its UUID is derived, not random, so
// every reader of the same documents computes the same tree.
var LocalFederation = Node{
	ID:       uuid.NewSHA1(uuid.NameSpaceURL, []byte("https://sparc.risk-sentinel.org/ns/horizon/local-federation")).String(),
	Name:     "Local federation",
	NodeType: Federation,
}

// Options adjust how documents are read. The zero value is the strict reading.
type Options struct {
	// RoleAliases maps a document's role-id onto one the contract enumerates,
	// for documents that name the same role differently (NIST's examples say
	// "owner" for the system owner). The mapping is the operator's statement,
	// not something inferred: an unmapped unknown role is still reported.
	RoleAliases map[string]Role
}

// Binding is one party holding one role at one node.
//
// It is kept BESIDE the tree rather than on Node, because Node is the frozen
// contract shape — `additionalProperties: false` — and a party UUID has no
// place in it. The API answers "what roles does this node carry"; authorization
// needs "which party holds them", and that is a different question asked by a
// different caller (#76).
//
// Without this the information is simply lost: it exists while the document is
// being read and nothing downstream can recover it, because a party that binds
// at an organization need not appear in every document beneath it.
type Binding struct {
	NodeID    string   `json:"nodeId"`
	NodeType  NodeType `json:"nodeType"`
	Role      Role     `json:"role"`
	PartyUUID string   `json:"partyUuid"`
}

// Result is the tree, who holds what on it, and everything that did not fit.
type Result struct {
	Root     *Node     `json:"root"`
	Bindings []Binding `json:"bindings,omitempty"`
	Findings []Finding `json:"findings,omitempty"`
}
