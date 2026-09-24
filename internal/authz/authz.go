// Package authz turns the role bindings a tree records into a decision.
//
// Two rules govern everything here, and both come from the frozen contract
// rather than from this package's convenience:
//
//  1. **A refusal is a 404.** A node the caller holds nothing on is
//     indistinguishable from a node that does not exist. There is deliberately
//     no Exists method: offering one would let a handler tell the two apart,
//     and the whole point (docs/07-security.md, api/openapi.yaml) is that
//     Horizon's tree spans organizations that are not meant to see each other.
//
//  2. **The caller sees a different tree**, rooted at the node they hold a role
//     on — not the whole tree with parts greyed out. That is what makes the
//     first rule coherent: there is nothing to grey out, because the parts were
//     never in the response.
//
// Roles inherit DOWNWARD. A party bound at an organization holds authority over
// every boundary and system beneath it, including boundaries whose documents
// never name that party — inheritance is by position in the tree, not by
// mention. The fixtures prove the distinction matters: the two ODS boundaries
// declare different authorizing officials, and each sees both.
//
// The caller is a PARTY UUID. docs/07-security.md says party UUIDs map to OIDC
// subjects and nothing specifies how — see docs/10-risks-decisions.md, which
// records that as an open decision rather than letting this package invent one.
package authz

import (
	"fmt"

	"github.com/risk-sentinel/sparc-horizon/internal/tree"
)

// Authorizer answers questions about one tree. It is read-only and safe to
// share once built.
type Authorizer struct {
	// byID is every node, so a decision can be made against the node a
	// request NAMES rather than against whatever the handler had to hand.
	byID map[string]*tree.Node
	// parent lets a subtree be rooted, and lets inheritance be walked upward
	// from the node in question rather than expanded eagerly.
	parent map[string]string
	// binds is party UUID -> node ids where that party holds a role.
	binds map[string][]string
	// roles is node id -> the roles bound there, per party.
	roles map[string]map[string][]tree.Role
}

// New indexes a built tree for authorization.
func New(res tree.Result) (*Authorizer, error) {
	if res.Root == nil {
		return nil, fmt.Errorf("authz: tree has no root")
	}

	a := &Authorizer{
		byID:   map[string]*tree.Node{},
		parent: map[string]string{},
		binds:  map[string][]string{},
		roles:  map[string]map[string][]tree.Role{},
	}
	a.index(res.Root, "")

	for _, b := range res.Bindings {
		if _, ok := a.byID[b.NodeID]; !ok {
			// A binding for a node that is not in the tree cannot be honoured,
			// and silently ignoring it would grant nothing while looking fine.
			return nil, fmt.Errorf("authz: binding for node %s, which is not in the tree", b.NodeID)
		}
		a.addBinding(b)
	}
	return a, nil
}

func (a *Authorizer) index(n *tree.Node, parent string) {
	a.byID[n.ID] = n
	if parent != "" {
		a.parent[n.ID] = parent
	}
	for _, c := range n.Children {
		a.index(c, n.ID)
	}
}

func (a *Authorizer) addBinding(b tree.Binding) {
	if !contains(a.binds[b.PartyUUID], b.NodeID) {
		a.binds[b.PartyUUID] = append(a.binds[b.PartyUUID], b.NodeID)
	}
	if a.roles[b.NodeID] == nil {
		a.roles[b.NodeID] = map[string][]tree.Role{}
	}
	have := a.roles[b.NodeID][b.PartyUUID]
	for _, r := range have {
		if r == b.Role {
			return
		}
	}
	a.roles[b.NodeID][b.PartyUUID] = append(have, b.Role)
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
