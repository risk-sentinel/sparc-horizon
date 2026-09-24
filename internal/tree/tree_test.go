package tree

import (
	"encoding/json"
	"io/fs"
	"os"
	"sort"
	"testing"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"
)

// The committed fixture federation, read from disk rather than regenerated.
// The exit criterion on #16 is that the FIXTURE tree matches the golden file,
// so the test has to build from the fixtures a reviewer can open.
//
// Read through an fs.FS rooted at the fixture directory rather than with
// os.ReadFile over a glob. The paths come from the directory listing, so
// nothing outside it is reachable by construction — which is the property
// gosec's G304 asks about, answered rather than excluded (.golangci.yml keeps
// an empty gosec exclusion set on purpose).
const (
	fixtureDir     = "../../fixtures/oscal"
	fixturePattern = "ssp-*.json"
)

func loadSSPs(t *testing.T) []*oscal.SystemSecurityPlan {
	t.Helper()

	fsys := os.DirFS(fixtureDir)
	names, err := fs.Glob(fsys, fixturePattern)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(names) == 0 {
		t.Fatalf("no SSP fixtures matched %s in %s", fixturePattern, fixtureDir)
	}
	sort.Strings(names)

	out := make([]*oscal.SystemSecurityPlan, 0, len(names))
	for _, p := range names {
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		var doc oscal.OscalCompleteSchema
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if doc.SystemSecurityPlan == nil {
			t.Fatalf("%s: not a system-security-plan", p)
		}
		out = append(out, doc.SystemSecurityPlan)
	}
	return out
}

func build(t *testing.T) Result {
	t.Helper()
	res, err := Build(loadSSPs(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return res
}

// Exit criterion on #16: the fixture tree matches the golden JSON.
//
// Regenerate deliberately with -update after reading the diff. A golden file
// that regenerates on every run records whatever the code did, which is not a
// test of anything.
func TestFixtureTreeMatchesGolden(t *testing.T) {
	res := build(t)

	got, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	const golden = "testdata/fixture-tree.json"
	if *update {
		if err := os.WriteFile(golden, got, 0o600); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Log("golden updated")
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create it): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("tree does not match %s; run with -update after reading the diff", golden)
	}
}

// Exit criterion on #16: AO at organization, SO and ISO at boundary.
//
// Asserted as the SHAPE OF EVERY TIER rather than by sampling one node,
// because the failure that matters is a role landing on the wrong tier — and
// one sampled node cannot distinguish "AO is on the organization" from "AO is
// on everything".
func TestRolesBindToTheRightTier(t *testing.T) {
	res := build(t)

	want := map[NodeType][]Role{
		Federation:   {},
		Organization: {AuthorizingOfficial},
		Boundary:     {ISSO, SystemOwner},
		System:       {SystemOwner},
	}
	counts := map[NodeType]int{}

	walk(res.Root, func(n *Node) {
		counts[n.NodeType]++
		expect, ok := want[n.NodeType]
		if !ok {
			t.Errorf("unexpected node type %q", n.NodeType)
			return
		}
		if !sameRoles(n.Roles, expect) {
			t.Errorf("%s %q carries %v, want %v", n.NodeType, n.Name, n.Roles, expect)
		}
	})

	// The federation's documented shape, so a tier silently collapsing is a
	// failure rather than a smaller green tree.
	for nt, n := range map[NodeType]int{Federation: 1, Organization: 4, Boundary: 7, System: 20} {
		if counts[nt] != n {
			t.Errorf("%d %s nodes, want %d", counts[nt], nt, n)
		}
	}
}

func TestFixturesProduceNoFindings(t *testing.T) {
	res := build(t)
	for _, f := range res.Findings {
		t.Errorf("unexpected finding: %s %s: %s", f.Kind, f.Source, f.Detail)
	}
}

func walk(n *Node, fn func(*Node)) {
	fn(n)
	for _, c := range n.Children {
		walk(c, fn)
	}
}

func sameRoles(got, want []Role) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
