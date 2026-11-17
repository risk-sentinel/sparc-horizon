package tree

import (
	"encoding/json"
	"os"
	"testing"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-3"
)

func nistSSP(t *testing.T) *oscal.SystemSecurityPlan {
	t.Helper()
	b, err := os.ReadFile("../oscal/testdata/nist/ifa_ssp-example.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc oscal.OscalCompleteSchema
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.SystemSecurityPlan
}

// NIST's example names one organization and no federation, which is the shape
// of most documents outside this estate. Before #120 it built no tree at all.
func TestLoneOrganizationGoesUnderTheLocalFederation(t *testing.T) {
	res, err := Build([]*oscal.SystemSecurityPlan{nistSSP(t)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if res.Root.ID != LocalFederation.ID {
		t.Fatalf("root is %s, want the local federation", res.Root.ID)
	}
	if kinds(res)[FindingImplicitFederation] != 1 {
		t.Errorf("the implicit federation was not reported: %+v", res.Findings)
	}
	if got := countType(res.Root, Boundary); got != 1 {
		t.Errorf("%d boundaries, want 1", got)
	}
	if countType(res.Root, System) == 0 {
		t.Error("no system nodes: NIST's inventory implements its component")
	}
}

// Its roles are "owner", "developer" and "system-engineer". None is one the
// contract enumerates, so without an alias nobody holds a role on it.
func TestRoleAliasesBindAndUnmappedRolesAreStillReported(t *testing.T) {
	ssp := nistSSP(t)

	strict, err := Build([]*oscal.SystemSecurityPlan{ssp})
	if err != nil {
		t.Fatal(err)
	}
	if len(strict.Bindings) != 0 {
		t.Fatalf("premise wrong: %d bindings without an alias", len(strict.Bindings))
	}

	res, err := BuildWith([]*oscal.SystemSecurityPlan{ssp}, Options{RoleAliases: map[string]Role{"owner": SystemOwner}})
	if err != nil {
		t.Fatal(err)
	}
	owners := 0
	for _, b := range res.Bindings {
		if b.Role == SystemOwner && b.NodeType == Boundary {
			owners++
		}
	}
	if owners != 1 {
		t.Errorf("%d system-owner bindings on the boundary, want 1: %+v", owners, res.Bindings)
	}
	if kinds(res)[FindingUnknownRole] == 0 {
		t.Error("developer and system-engineer are unmapped and must still be reported")
	}
}
