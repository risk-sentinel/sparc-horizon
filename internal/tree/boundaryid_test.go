package tree

import (
	"testing"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-3"

	"github.com/risk-sentinel/sparc-horizon/internal/keys"
)

const sparcBoundary = "6a1d2f8e-3c5b-4e7a-9f10-2b3c4d5e6f70"

func withProps(ssp *oscal.SystemSecurityPlan, docUUID string, props ...oscal.Property) *oscal.SystemSecurityPlan {
	c := *ssp
	c.UUID = docUUID
	ps := append([]oscal.Property{}, props...)
	c.Metadata.Props = &ps
	return &c
}

func sparcProp(name, value string) oscal.Property {
	return oscal.Property{Name: name, Ns: keys.NamespaceURI, Value: value}
}

func boundaryIDs(t *testing.T, res Result) []string {
	t.Helper()
	var out []string
	var walk func(n *Node)
	walk = func(n *Node) {
		if n.NodeType == Boundary {
			out = append(out, n.ID)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(res.Root)
	return out
}

// SPARC's form keys the boundary node by SPARC's boundary uuid, and a revision
// that changes the document uuid does not move it.
func TestASPARCSSPIsKeyedByItsBoundaryUUID(t *testing.T) {
	for _, doc := range []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"} {
		ssp := withProps(nistSSP(t), doc, sparcProp("node-type", "system"), sparcProp("parent-uuid", sparcBoundary))
		res, err := Build([]*oscal.SystemSecurityPlan{ssp})
		if err != nil {
			t.Fatal(err)
		}
		if got := boundaryIDs(t, res); len(got) != 1 || got[0] != sparcBoundary {
			t.Fatalf("document %s: boundary ids %v, want [%s]", doc, got, sparcBoundary)
		}
		for _, bd := range res.Bindings {
			if bd.NodeType == Boundary && bd.NodeID != sparcBoundary {
				t.Errorf("a boundary role is bound to %s, not the boundary's id", bd.NodeID)
			}
		}
	}
}

// Every other form keeps the document uuid: Horizon's fixtures (node-type
// boundary, parent-uuid naming the organization), a parent-uuid in another
// namespace, and node-type system without a parent.
func TestOtherFormsKeepTheDocumentUUID(t *testing.T) {
	const doc = "33333333-3333-4333-8333-333333333333"
	cases := map[string][]oscal.Property{
		"fixture form": {sparcProp("node-type", "boundary"), sparcProp("parent-uuid", sparcBoundary)},
		"foreign ns": {
			{Name: "node-type", Ns: "http://example.com/ns", Value: "system"},
			{Name: "parent-uuid", Ns: "http://example.com/ns", Value: sparcBoundary},
		},
		"no namespace":    {{Name: "node-type", Value: "system"}, {Name: "parent-uuid", Value: sparcBoundary}},
		"no parent":       {sparcProp("node-type", "system")},
		"no node-type":    {sparcProp("parent-uuid", sparcBoundary)},
		"no props at all": nil,
	}
	for name, props := range cases {
		ssp := withProps(nistSSP(t), doc, props...)
		if props == nil {
			ssp.Metadata.Props = nil
		}
		res, err := Build([]*oscal.SystemSecurityPlan{ssp})
		if err != nil {
			t.Fatal(err)
		}
		if got := boundaryIDs(t, res); len(got) != 1 || got[0] != doc {
			t.Errorf("%s: boundary ids %v, want [%s]", name, got, doc)
		}
	}
}

// Two SSPs naming one boundary are two boundaries in the tree, each under its
// own document uuid, and the claim is reported rather than merged or dropped.
func TestAContestedBoundaryUUIDIsReported(t *testing.T) {
	a := withProps(nistSSP(t), "44444444-4444-4444-8444-444444444444",
		sparcProp("node-type", "system"), sparcProp("parent-uuid", sparcBoundary))
	b := withProps(nistSSP(t), "55555555-5555-4555-8555-555555555555",
		sparcProp("node-type", "system"), sparcProp("parent-uuid", sparcBoundary))
	b.SystemCharacteristics.SystemIds = nil // not also a duplicate system-id
	res, err := Build([]*oscal.SystemSecurityPlan{a, b})
	if err != nil {
		t.Fatal(err)
	}
	got := boundaryIDs(t, res)
	if len(got) != 2 || got[0] == got[1] || got[0] == sparcBoundary || got[1] == sparcBoundary {
		t.Fatalf("boundary ids %v, want both document uuids", got)
	}
	if kinds(res)[FindingDuplicateBoundary] != 1 {
		t.Errorf("the contested boundary was not reported once: %+v", res.Findings)
	}
}
