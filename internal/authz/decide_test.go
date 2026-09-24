package authz

import (
	"testing"

	"github.com/risk-sentinel/sparc-horizon/internal/tree"
)

const (
	aoODS        = "377df8c6-c8fa-5217-a2ff-445247a7b7c5" // AO declared in ssp-ods-portal
	soODSPortal  = "f935a0b4-4974-5aec-a8ed-590328a8fa6d"
	odsOrg       = "cdf7d9f6-cf10-5944-a0ef-0351bf1ec41a"
	portalSSP    = "f7de308a-5555-5e35-be6a-029ad885320e"
	nonexistent  = "00000000-0000-5000-8000-000000000000"
	identitySSPQ = "Identity Services"
)

// The crux of the slice, and the way it is most likely to be built wrong.
//
// ssp-ods-portal and ssp-ods-identity declare DIFFERENT authorizing officials.
// The ao-ods party appears nowhere in the identity SSP, and must still see that
// boundary and its systems: roles inherit downward from the organization, so
// visibility is by POSITION IN THE TREE, not by being mentioned in a document.
//
// An implementation that collected "documents naming this party" would return a
// smaller tree and pass every other test here.
func TestInheritanceIsByPositionNotByMention(t *testing.T) {
	a := authorizer(t)

	identity := findNode(t, a, identitySSPQ)
	if !a.Visible(aoODS, identity.ID) {
		t.Fatalf("the AO cannot see %q, whose SSP never names it — inheritance is not being applied", identitySSPQ)
	}
	if len(identity.Children) == 0 {
		t.Fatal("the identity boundary has no systems; the fixture changed")
	}
	for _, sys := range identity.Children {
		if !a.Visible(aoODS, sys.ID) {
			t.Errorf("the AO cannot see system %q beneath a boundary it can see", sys.Name)
		}
	}

	// And the role is the inherited one, not one declared down there.
	roles := a.Roles(aoODS, identity.ID)
	if len(roles) != 1 || roles[0] != tree.AuthorizingOfficial {
		t.Errorf("effective roles at %q are %v, want [authorizing-official]", identitySSPQ, roles)
	}
}

// Exit criterion: a node outside the caller's subtree is indistinguishable from
// one that does not exist. Same value, same call, no second method that could
// tell them apart.
func TestRefusalIsIndistinguishableFromAbsence(t *testing.T) {
	a := authorizer(t)

	// A real node the caller holds nothing on: the ODS organization, seen by a
	// system owner bound one tier below it.
	if a.Visible(soODSPortal, odsOrg) {
		t.Error("a system owner can see the organization above its boundary; roles inherit DOWNWARD only")
	}
	if got := a.Roles(soODSPortal, odsOrg); got != nil {
		t.Errorf("roles above the binding are %v, want none", got)
	}

	// A node that does not exist at all.
	if a.Visible(soODSPortal, nonexistent) {
		t.Error("a nonexistent node reported visible")
	}
	if got := a.Roles(soODSPortal, nonexistent); got != nil {
		t.Errorf("roles on a nonexistent node are %v, want none", got)
	}

	// The two are the same answer. If they ever differ, a handler can tell a
	// caller that a node exists by refusing it differently.
	if a.Visible(soODSPortal, odsOrg) != a.Visible(soODSPortal, nonexistent) {
		t.Error("an unauthorized node and an absent node answer differently")
	}
}

// Two personas in different organizations must not overlap at all.
func TestOrganizationsDoNotOverlap(t *testing.T) {
	a := authorizer(t)

	bgm := ""
	for _, p := range personas(t) {
		if p.ID == "iso-bgm-grants" {
			bgm = p.PartyUUID
		}
	}
	if bgm == "" {
		t.Fatal("iso-bgm-grants is not in personas.json")
	}

	sub := a.Subtree(soODSPortal)
	if sub == nil {
		t.Fatal("no subtree for the portal system owner")
	}
	walkNodes(sub, func(n *tree.Node) {
		if a.Visible(bgm, n.ID) {
			t.Errorf("the BGM ISSO can see %s %q, which belongs to ODS", n.NodeType, n.Name)
		}
	})
}

// A caller holding nothing gets nil, not an empty tree. An empty tree still
// asserts that a tree exists and that the caller is entitled to ask.
func TestCallerWithNoBindingsGetsNothing(t *testing.T) {
	a := authorizer(t)

	if got := a.Subtree(nonexistent); got != nil {
		t.Errorf("a caller with no bindings got a subtree rooted at %s", got.ID)
	}
	if a.Visible(nonexistent, portalSSP) {
		t.Error("a caller with no bindings can see a node")
	}
}

// A binding for a node that is not in the tree must refuse rather than be
// ignored: silently dropping it grants nothing while looking healthy.
func TestBindingForAnAbsentNodeIsRefused(t *testing.T) {
	res := treeResult(t)
	res.Bindings = append(res.Bindings, tree.Binding{
		NodeID: nonexistent, NodeType: tree.Boundary, Role: tree.SystemOwner, PartyUUID: aoODS,
	})
	if _, err := New(res); err == nil {
		t.Error("New accepted a binding for a node that is not in the tree")
	}
}

func TestNewRefusesATreeWithNoRoot(t *testing.T) {
	if _, err := New(tree.Result{}); err == nil {
		t.Error("New accepted a tree with no root")
	}
}

func findNode(t *testing.T, a *Authorizer, name string) *tree.Node {
	t.Helper()
	for _, n := range a.byID {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("no node named %q", name)
	return nil
}

func walkNodes(n *tree.Node, fn func(*tree.Node)) {
	fn(n)
	for _, c := range n.Children {
		walkNodes(c, fn)
	}
}
