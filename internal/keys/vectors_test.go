package keys

import (
	"testing"

	"github.com/risk-sentinel/sparc-horizon/internal/canonical"
)

// deriveFromArgs is the recipe a port follows to consume the vector file: read
// the arguments as issued, call the entry point for the object kind, compare
// the canonical fields and the UUID. Driving the Go tests through it as well
// keeps the file executable rather than decorative.
func deriveFromArgs(d Deriver, kind string, a Args) (Key, error) {
	vocab, err := canonical.ParseVocabulary(a.Vocabulary)
	if err != nil {
		return Key{}, err
	}
	src := Source{UUID: a.SourceUUID, Vocabulary: vocab}

	switch Kind(kind) {
	case KindAttestation:
		return d.Attestation(a.ParentSSPUUID, src, a.ControlID, a.ComponentUUID, a.Period)
	case KindObservation:
		return d.Observation(a.ParentSSPUUID, src, a.ControlID, a.ComponentUUID, a.Period)
	case KindFinding:
		return d.Finding(a.ParentSSPUUID, src, a.ControlID, a.ComponentUUID, a.Period)
	case KindRisk:
		return d.Risk(a.ParentSSPUUID, src, a.ControlID, a.ComponentUUID, a.Period)
	case KindPOAMItem:
		return d.POAMItem(a.ParentSSPUUID, src, a.ControlID, a.ComponentUUID, a.Period)
	case KindResource:
		return d.EvidenceResource(a.ParentSSPUUID, a.SHA256)
	case KindDecision:
		return d.AODecision(a.ParentSSPUUID, a.RiskUUID, a.Period)
	case KindResponsibility:
		return d.Responsibility(a.ParentSSPUUID, src, a.ControlID, a.ComponentUUID, Half(a.Half))
	case KindCell:
		if a.FamilyID != "" {
			return d.CellForFamily(a.NodeUUID, src, a.FamilyID, Bucket(a.HorizonBucket))
		}
		return d.CellForControl(a.NodeUUID, src, a.ControlID, Bucket(a.HorizonBucket))
	default:
		return Key{}, errUnknownKind(kind)
	}
}

type errUnknownKind string

func (e errUnknownKind) Error() string { return "unknown object kind: " + string(e) }

func vectorDoc(t *testing.T) VectorDocument {
	t.Helper()
	doc, err := Vectors(ProvisionalNamespace())
	if err != nil {
		t.Fatalf("Vectors: %v", err)
	}
	return doc
}

// Every vector recomputes from the arguments it publishes. Without this the
// file would only record what the generator happened to produce.
func TestVectorsRecomputeFromTheirArguments(t *testing.T) {
	doc := vectorDoc(t)
	d := New(ProvisionalNamespace())

	if len(doc.Vectors) == 0 {
		t.Fatal("no vectors emitted")
	}
	seen := map[string]bool{}
	for _, v := range doc.Vectors {
		if seen[v.Name] {
			t.Errorf("duplicate vector name %q", v.Name)
		}
		seen[v.Name] = true

		k, err := deriveFromArgs(d, v.Kind, v.Args)
		if err != nil {
			t.Errorf("%s: %v", v.Name, err)
			continue
		}
		if k.UUID.String() != v.UUID {
			t.Errorf("%s: recomputed %s, published %s", v.Name, k.UUID, v.UUID)
		}
		if len(k.Fields) != len(v.CanonicalFields) {
			t.Errorf("%s: %d canonical fields, published %d", v.Name, len(k.Fields), len(v.CanonicalFields))
			continue
		}
		for i := range k.Fields {
			if k.Fields[i] != v.CanonicalFields[i] {
				t.Errorf("%s: field %d recomputed %q, published %q", v.Name, i, k.Fields[i], v.CanonicalFields[i])
			}
		}
	}
}

// The assertions are the part a port cannot fake by storing the UUIDs: they
// state relations that must hold however the values were reached.
func TestVectorAssertionsHold(t *testing.T) {
	doc := vectorDoc(t)
	byName := map[string]Vector{}
	for _, v := range doc.Vectors {
		byName[v.Name] = v
	}
	if len(doc.Assertions) == 0 {
		t.Fatal("no assertions emitted")
	}
	for _, a := range doc.Assertions {
		checkAssertion(t, a, byName)
	}
}

// checkAssertion resolves one assertion's vectors and applies its relation.
func checkAssertion(t *testing.T, a Assertion, byName map[string]Vector) {
	t.Helper()

	if len(a.Vectors) < 2 {
		t.Errorf("%s: an assertion over %d vectors states nothing", a.Name, len(a.Vectors))
	}
	if a.Why == "" {
		t.Errorf("%s: no rationale", a.Name)
	}
	if a.Relation != "same" && a.Relation != "distinct" {
		t.Errorf("%s: unknown relation %q", a.Name, a.Relation)
		return
	}

	// uuid -> the first vector that derived it.
	derivedBy := map[string]string{}
	for _, name := range a.Vectors {
		v, ok := byName[name]
		if !ok {
			t.Errorf("%s: references unknown vector %q", a.Name, name)
			continue
		}
		if other, clash := derivedBy[v.UUID]; clash && a.Relation == "distinct" {
			t.Errorf("%s: %q and %q both derived %s", a.Name, other, name, v.UUID)
		}
		derivedBy[v.UUID] = name
	}
	if a.Relation == "same" && len(derivedBy) != 1 {
		t.Errorf("%s: %d distinct UUIDs in a group that must agree: %v", a.Name, len(derivedBy), derivedBy)
	}
}

// Every published rejection must actually be refused. A port that derives a
// UUID here has a defect no equality check would find.
func TestVectorRejectionsAreRefused(t *testing.T) {
	doc := vectorDoc(t)
	d := New(ProvisionalNamespace())

	if len(doc.Rejections) == 0 {
		t.Fatal("no rejections emitted")
	}
	for _, r := range doc.Rejections {
		if r.Why == "" {
			t.Errorf("%s: no rationale", r.Name)
		}
		if k, err := deriveFromArgs(d, r.Kind, r.Args); err == nil {
			t.Errorf("%s: derived %s instead of refusing", r.Name, k.UUID)
		}
	}
}

// The defect the separator change fixed (#30), carried in the file so the
// ports inherit the regression rather than the conclusion.
func TestJoinCasesDoNotCollide(t *testing.T) {
	doc := vectorDoc(t)
	if len(doc.JoinCases) != 2 {
		t.Fatalf("got %d join cases, want the collision pair", len(doc.JoinCases))
	}
	if doc.JoinCases[0].UUID == doc.JoinCases[1].UUID {
		t.Errorf("the join cases still collide: %s", doc.JoinCases[0].UUID)
	}
	if doc.Separator != canonical.Separator {
		t.Errorf("published separator %q is not the one the grammar joins on", doc.Separator)
	}
	if doc.Grammar != GrammarVersion {
		t.Errorf("published grammar %q", doc.Grammar)
	}
	if !doc.Namespace.Provisional {
		t.Error("vectors derived under the provisional namespace must say so")
	}
	if doc.Namespace.UUID != ProvisionalNamespace().String() {
		t.Errorf("published namespace %s", doc.Namespace.UUID)
	}
}
