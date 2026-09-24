package authz

import (
	"sort"

	"github.com/risk-sentinel/sparc-horizon/internal/tree"
)

// Visible reports whether the caller may see the node the request NAMED.
//
// This is the whole of TM-1's third requirement: the decision takes a node id,
// so a handler cannot authorize the endpoint it reached instead of the node it
// is about. There is no variant that takes a path, a resource kind or a
// session — those are the shapes that let the check drift away from the object.
//
// A node that does not exist and a node the caller holds nothing on return the
// SAME value through the SAME path. Nothing above this can tell them apart,
// because there is nothing here that knows the difference.
func (a *Authorizer) Visible(party, nodeID string) bool {
	if _, ok := a.byID[nodeID]; !ok {
		return false
	}
	return a.holdsAtOrAbove(party, nodeID)
}

// holdsAtOrAbove walks from the node toward the root. Inheritance is DOWNWARD,
// so a binding anywhere on the path from this node up to the root grants it —
// which is the same statement read from the other end.
//
// Walking up rather than expanding down is what keeps a party bound at an
// organization able to see a boundary whose document never mentions it.
func (a *Authorizer) holdsAtOrAbove(party, nodeID string) bool {
	for id := nodeID; id != ""; id = a.parent[id] {
		for _, bound := range a.binds[party] {
			if bound == id {
				return true
			}
		}
	}
	return false
}

// Roles returns the caller's EFFECTIVE roles at a node: bound there, or
// inherited from any ancestor. internal/tree deliberately records only what
// each node declares (#74); this is where inheritance is applied, and keeping
// the two apart is what lets the tree stay a faithful projection of the
// documents.
//
// A node the caller cannot see yields nothing, for the same reason Visible
// does: a caller must not learn a node exists by being told it has no roles
// on it rather than that it is absent.
func (a *Authorizer) Roles(party, nodeID string) []tree.Role {
	if !a.Visible(party, nodeID) {
		return nil
	}

	seen := map[tree.Role]bool{}
	for id := nodeID; id != ""; id = a.parent[id] {
		for _, r := range a.roles[id][party] {
			seen[r] = true
		}
	}

	out := make([]tree.Role, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Subtree returns the caller's own tree, or nil if they hold nothing anywhere.
//
// Rooted at the node the caller binds to, NOT at the federation with parts
// removed — api/openapi.yaml: "a caller sees a different tree rather than the
// same tree with parts greyed out". A nil return is the honest answer for a
// caller with no bindings: an empty tree would still assert that a tree exists.
//
// The returned nodes are COPIES. Handing back pointers into the shared tree
// would let a caller's response be mutated into another caller's, and the
// sharing would not be visible at the call site.
func (a *Authorizer) Subtree(party string) *tree.Node {
	root := a.rootFor(party)
	if root == nil {
		return nil
	}
	return a.copyVisible(root, party)
}

// rootFor picks the node a caller's tree hangs from: the highest node they
// bind to. Where bindings are disjoint — a party holding roles in two
// organizations — there is no single node that contains them, and the contract
// returns exactly one Node. That case cannot arise in the fixtures and is
// recorded as an open decision rather than resolved by guessing here; until it
// is settled, the highest binding wins and the others are unreachable, which
// is the conservative direction: it shows too little, never too much.
func (a *Authorizer) rootFor(party string) *tree.Node {
	bound := a.binds[party]
	if len(bound) == 0 {
		return nil
	}

	best, bestDepth := "", -1
	for _, id := range bound {
		if d := a.depth(id); bestDepth == -1 || d < bestDepth {
			best, bestDepth = id, d
		}
	}
	return a.byID[best]
}

func (a *Authorizer) depth(id string) int {
	d := 0
	for cur := id; a.parent[cur] != ""; cur = a.parent[cur] {
		d++
	}
	return d
}

// copyVisible deep-copies the subtree, keeping only what the caller may see.
//
// `roles` on each node is the CALLER'S effective roles there, not every role
// bound at that node. The frozen persona goldens settle it — so-ods-portal's
// boundary carries only `system-owner`, although an ISSO is bound there too —
// and it is the right answer twice over: the HUD asks "what may I do here",
// and listing every role holder would disclose an organization's staffing to
// anyone who can see the node.
//
// internal/tree keeps the full set, because the documents declare it and the
// recompute audit needs it. The two answers are different questions.
func (a *Authorizer) copyVisible(n *tree.Node, party string) *tree.Node {
	roles := a.Roles(party, n.ID)
	if roles == nil {
		roles = []tree.Role{}
	}
	out := &tree.Node{
		ID:       n.ID,
		Name:     n.Name,
		NodeType: n.NodeType,
		Roles:    roles,
	}
	for _, c := range n.Children {
		if a.holdsAtOrAbove(party, c.ID) {
			out.Children = append(out.Children, a.copyVisible(c, party))
		}
	}
	return out
}
