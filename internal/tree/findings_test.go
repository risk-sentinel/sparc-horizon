package tree

import (
	"encoding/json"
	"math/rand"
	"testing"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"
)

// clone deep-copies an SSP so a test can break one document without breaking
// the fixture on disk or the other tests in this package.
func clone(t *testing.T, in *oscal.SystemSecurityPlan) *oscal.SystemSecurityPlan {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out oscal.SystemSecurityPlan
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &out
}

func kinds(res Result) map[string]int {
	out := map[string]int{}
	for _, f := range res.Findings {
		out[f.Kind]++
	}
	return out
}

func countType(root *Node, nt NodeType) int {
	n := 0
	walk(root, func(x *Node) {
		if x.NodeType == nt {
			n++
		}
	})
	return n
}

// Every one of these asserts the SAME property: the defect is REPORTED and the
// rest of the tree still builds. Task 5 on #16 is "report orphaned or
// inconsistent parties instead of dropping them", and a builder that quietly
// skipped the bad row would pass a test that only checked the tree was valid.

func TestUnknownPartyIsReported(t *testing.T) {
	ssps := loadSSPs(t)
	bad := clone(t, ssps[0])
	rps := *bad.Metadata.ResponsibleParties
	rps[0].PartyUuids = []string{"11111111-1111-5111-8111-111111111111"}
	*bad.Metadata.ResponsibleParties = rps
	ssps[0] = bad

	res, err := Build(ssps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if kinds(res)[FindingUnknownParty] == 0 {
		t.Error("a responsible-party naming an undeclared UUID was not reported")
	}
	if got := countType(res.Root, Boundary); got != 7 {
		t.Errorf("%d boundaries, want 7 — the document was dropped rather than reported", got)
	}
}

func TestUnknownRoleIsReportedAndNotCarried(t *testing.T) {
	ssps := loadSSPs(t)
	bad := clone(t, ssps[0])
	rps := *bad.Metadata.ResponsibleParties
	rps[0].RoleId = "chief-of-vibes"
	*bad.Metadata.ResponsibleParties = rps
	ssps[0] = bad

	res, err := Build(ssps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if kinds(res)[FindingUnknownRole] == 0 {
		t.Error("a role-id outside the contract's enum was not reported")
	}
	// The contract's enum is closed, so the value must not reach the response.
	walk(res.Root, func(n *Node) {
		for _, r := range n.Roles {
			if _, ok := knownRole(string(r)); !ok {
				t.Errorf("%s %q carries role %q, which the schema rejects", n.NodeType, n.Name, r)
			}
		}
	})
}

func TestOrphanInventoryItemIsReported(t *testing.T) {
	ssps := loadSSPs(t)
	bad := clone(t, ssps[0])
	items := *bad.SystemImplementation.InventoryItems
	ics := *items[0].ImplementedComponents
	ics[0].ComponentUuid = "22222222-2222-5222-8222-222222222222"
	*items[0].ImplementedComponents = ics
	*bad.SystemImplementation.InventoryItems = items
	ssps[0] = bad

	res, err := Build(ssps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if kinds(res)[FindingOrphanInventory] == 0 {
		t.Error("an inventory item implementing an undeclared component was not reported")
	}
	if got := countType(res.Root, System); got != 20 {
		t.Errorf("%d systems, want 20 — the item was dropped rather than reported", got)
	}
}

func TestDuplicateSystemIDIsReported(t *testing.T) {
	ssps := loadSSPs(t)
	bad := clone(t, ssps[1])
	bad.SystemCharacteristics.SystemIds[0].ID = ssps[0].SystemCharacteristics.SystemIds[0].ID
	ssps[1] = bad

	res, err := Build(ssps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if kinds(res)[FindingDuplicateSystemID] == 0 {
		t.Error("two SSPs claiming one system-id were not reported")
	}
	if got := countType(res.Root, Boundary); got != 7 {
		t.Errorf("%d boundaries, want 7 — a document was dropped rather than reported", got)
	}
}

func TestFederationSplitIsReported(t *testing.T) {
	ssps := loadSSPs(t)
	bad := clone(t, ssps[0])
	parties := *bad.Metadata.Parties
	for i, p := range parties {
		if p.Type == "organization" && len(memberOf(p)) == 0 {
			parties[i].UUID = "33333333-3333-5333-8333-333333333333"
		}
	}
	*bad.Metadata.Parties = parties
	ssps[0] = bad

	res, err := Build(ssps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Whichever federation wins the root, the disagreement must be visible:
	// joining two federations into one tree would invent a structure neither
	// document describes.
	if kinds(res)[FindingFederationSplit] == 0 && kinds(res)[FindingNoOrganization] == 0 {
		t.Error("an SSP naming a different federation was neither reported nor refused")
	}
}

func TestBuildRefusesAnEmptySet(t *testing.T) {
	if _, err := Build(nil); err == nil {
		t.Error("Build accepted an empty SSP set; an empty tree is indistinguishable from a failed load")
	}
}

// The tree must not depend on the order documents arrived in, or the golden
// file is testing the filesystem rather than the builder.
func TestBuildIsOrderIndependent(t *testing.T) {
	first, err := Build(loadSSPs(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for i := 0; i < 5; i++ {
		shuffled := loadSSPs(t)
		rand.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })

		res, err := Build(shuffled)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		got, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(got) != string(want) {
			t.Fatal("the tree changed with the order the SSPs were loaded in")
		}
	}
}

// A document missing the optional assemblies must degrade into findings, not
// panic and not silently produce a smaller tree. go-oscal models every
// optional assembly as a pointer, so "absent" and "empty" are different
// values, and a real export may omit any of them.
func TestDocumentsMissingOptionalAssemblies(t *testing.T) {
	ssps := loadSSPs(t)
	stripped := clone(t, ssps[0])
	stripped.Metadata.ResponsibleParties = nil
	stripped.SystemImplementation.InventoryItems = nil
	stripped.SystemImplementation.Components = nil
	ssps[0] = stripped

	res, err := Build(ssps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// The boundary is still placed: it has an organization, which is all
	// placement needs. It simply carries no roles and no systems.
	if got := countType(res.Root, Boundary); got != 7 {
		t.Errorf("%d boundaries, want 7", got)
	}
	if got := countType(res.Root, System); got != 17 {
		t.Errorf("%d systems, want 17 — the stripped document had 3", got)
	}
}

// An SSP with no organization party cannot be placed. It must be reported and
// the rest of the federation must still build, because one unplaceable
// document is not a reason to have no tree.
func TestDocumentWithNoOrganizationIsReported(t *testing.T) {
	ssps := loadSSPs(t)
	bad := clone(t, ssps[0])
	kept := []oscal.Party{}
	for _, p := range *bad.Metadata.Parties {
		if p.Type == "organization" && len(memberOf(p)) > 0 {
			continue // drop the organization, keep the federation
		}
		kept = append(kept, p)
	}
	*bad.Metadata.Parties = kept
	ssps[0] = bad

	res, err := Build(ssps)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if kinds(res)[FindingNoOrganization] == 0 {
		t.Error("an SSP with no organization party was not reported")
	}
	if got := countType(res.Root, Boundary); got != 6 {
		t.Errorf("%d boundaries, want 6 — the other six must still build", got)
	}
}

// No federation party anywhere means there is no root, which is a refusal
// rather than a finding: an empty tree and a failed load look identical to a
// caller, and only one of them is safe to serve.
func TestNoFederationPartyRefuses(t *testing.T) {
	ssps := loadSSPs(t)
	for i := range ssps {
		bad := clone(t, ssps[i])
		kept := []oscal.Party{}
		for _, p := range *bad.Metadata.Parties {
			if p.Type == "organization" && len(memberOf(p)) == 0 {
				continue
			}
			kept = append(kept, p)
		}
		*bad.Metadata.Parties = kept
		ssps[i] = bad
	}

	if _, err := Build(ssps); err == nil {
		t.Error("Build returned a tree with no federation party to root it")
	}
}

func TestNilSSPIsRefused(t *testing.T) {
	if _, err := Build([]*oscal.SystemSecurityPlan{nil}); err == nil {
		t.Error("Build accepted a nil SSP")
	}
}
