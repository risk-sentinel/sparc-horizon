package keys

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/risk-sentinel/sparc-horizon/internal/canonical"
)

const (
	testSSP       = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	testCatalog   = "b7e21d90-4c1a-4f55-9e33-0a6d2c118f44"
	testComponent = "9f1c0f4e-2b7a-4d61-8f52-1c9a3b7d4e60"
)

func nist() Source {
	return Source{UUID: testCatalog, Vocabulary: canonical.VocabNIST80053}
}

// keyer returns a helper that fails the test on a derivation error, so the
// tests read as the sequence of derivations they are.
func keyer(t *testing.T) func(Key, error) Key {
	t.Helper()
	return func(k Key, err error) Key {
		t.Helper()
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		return k
	}
}

// The grammar version is hashed in, so a future v2 cannot be mistaken for a v1
// identifier. If this ever fails silently the estate has two meanings for one
// UUID.
func TestGrammarVersionLeadsEveryKey(t *testing.T) {
	must := keyer(t)
	d := New(Namespace())
	k := must(d.Attestation(testSSP, nist(), "cp-4", testComponent, "2026-Q3"))

	if k.Fields[0] != GrammarVersion {
		t.Errorf("first field = %q, want the grammar version %q", k.Fields[0], GrammarVersion)
	}
	if !strings.HasPrefix(k.Input(), GrammarVersion+canonical.Separator) {
		t.Errorf("input does not begin with the grammar version: %q", k.Input())
	}
	if k.Input() != strings.Join(k.Fields, canonical.Separator) {
		t.Errorf("Input() = %q, want the fields joined with the unit separator", k.Input())
	}
	if k.Kind != KindAttestation {
		t.Errorf("Kind = %q", k.Kind)
	}
	if k.String() != k.UUID.String() {
		t.Errorf("String() = %q, want %q", k.String(), k.UUID.String())
	}
}

// The worked example in docs/03-data-model.md, field for field.
func TestWorkedExampleFieldOrder(t *testing.T) {
	must := keyer(t)
	d := New(Namespace())
	k := must(d.Attestation(testSSP, nist(), "CP-4", testComponent, "2026-Q3"))

	want := []string{GrammarVersion, testSSP, "attestation", testCatalog, "cp-4", testComponent, "2026-Q3"}
	if len(k.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d: %q", len(k.Fields), len(want), k.Fields)
	}
	for i := range want {
		if k.Fields[i] != want[i] {
			t.Errorf("field %d = %q, want %q", i, k.Fields[i], want[i])
		}
	}
}

// A key identifies a thing, not an assertion about a thing: the namespace is
// the only thing outside the field list that changes an identifier.
func TestNamespaceChangesEveryIdentifier(t *testing.T) {
	must := keyer(t)
	a := New(Namespace())
	b := New(uuid.MustParse("00000000-0000-5000-8000-000000000001"))

	ka := must(a.Observation(testSSP, nist(), "cp-4", testComponent, "2026-Q3"))
	kb := must(b.Observation(testSSP, nist(), "cp-4", testComponent, "2026-Q3"))

	if ka.Input() != kb.Input() {
		t.Error("the hashed input must not depend on the namespace")
	}
	if ka.UUID == kb.UUID {
		t.Error("two namespaces derived the same identifier")
	}
	if a.Namespace() != Namespace() {
		t.Error("Namespace() does not report the namespace it derives under")
	}
}

// The registered namespace is reachable from the published URI alone, so a
// port never has to copy a magic constant correctly — which is the property
// sparc#1155 chose a derived UUID for.
func TestNamespaceIsDerivedFromThePublishedURI(t *testing.T) {
	want := uuid.NewSHA1(uuid.NameSpaceURL, []byte(NamespaceURI))
	if got := Namespace(); got != want {
		t.Errorf("Namespace() = %s, want %s", got, want)
	}
	if Namespace().Version() != 5 {
		t.Error("the namespace must itself be a UUIDv5")
	}
	if got := Namespace().String(); got != "9f434272-f796-589b-b972-954790395630" {
		t.Errorf("Namespace() = %s, want the value registered in sparc#1155", got)
	}
}

func TestDerivationIsIdempotent(t *testing.T) {
	must := keyer(t)
	d := New(Namespace())
	first := must(d.Risk(testSSP, nist(), "sc-7", testComponent, "2026-Q3"))
	second := must(d.Risk(testSSP, nist(), "sc-7", testComponent, "2026-Q3"))
	if first.UUID != second.UUID {
		t.Error("the same inputs derived two identifiers")
	}
}

func TestEvidenceResourceIsKeyedOnContent(t *testing.T) {
	must := keyer(t)
	d := New(Namespace())
	const digest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	lower := must(d.EvidenceResource(testSSP, digest))
	upper := must(d.EvidenceResource(testSSP, strings.ToUpper(digest)))
	if lower.UUID != upper.UUID {
		t.Error("digest case changed the identifier; the same bytes are the same resource")
	}
	// Grammar version, parent SSP, kind token, digest — and nothing else: no
	// source-uuid, because bytes are not scoped to a catalog, and no period.
	if len(lower.Fields) != 4 {
		t.Errorf("evidence resource has %d fields, want 4: %q", len(lower.Fields), lower.Fields)
	}

	for _, bad := range []string{"deadbeef", "", digest + "00", "g" + digest[1:]} {
		if _, err := d.EvidenceResource(testSSP, bad); err == nil {
			t.Errorf("EvidenceResource accepted %q as a digest", bad)
		}
	}
}

func TestSourceUUIDIsRequiredRatherThanEmpty(t *testing.T) {
	d := New(Namespace())
	unresolved := Source{Vocabulary: canonical.VocabNIST80053}

	for name, fn := range map[string]func() (Key, error){
		"attestation":    func() (Key, error) { return d.Attestation(testSSP, unresolved, "cp-4", testComponent, "2026-Q3") },
		"observation":    func() (Key, error) { return d.Observation(testSSP, unresolved, "cp-4", testComponent, "2026-Q3") },
		"finding":        func() (Key, error) { return d.Finding(testSSP, unresolved, "cp-4", testComponent, "2026-Q3") },
		"risk":           func() (Key, error) { return d.Risk(testSSP, unresolved, "cp-4", testComponent, "2026-Q3") },
		"poam-item":      func() (Key, error) { return d.POAMItem(testSSP, unresolved, "cp-4", testComponent, "2026-Q3") },
		"responsibility": func() (Key, error) { return d.Responsibility(testSSP, unresolved, "cp-4", testComponent, HalfProvider) },
		"cell":           func() (Key, error) { return d.CellForControl(testSSP, unresolved, "cp-4", BucketToday) },
	} {
		if _, err := fn(); err == nil {
			t.Errorf("%s: derived without a resolved source-uuid", name)
		}
	}
}

func TestMalformedFieldsAreRejected(t *testing.T) {
	d := New(Namespace())
	src := nist()

	cases := map[string]func() (Key, error){
		"parent ssp is not a uuid": func() (Key, error) {
			return d.Observation("boundary-7", src, "cp-4", testComponent, "2026-Q3")
		},
		"component is not a uuid": func() (Key, error) {
			return d.Observation(testSSP, src, "cp-4", "web-01", "2026-Q3")
		},
		"period is not a period": func() (Key, error) {
			return d.Observation(testSSP, src, "cp-4", testComponent, "2026Q3")
		},
		"control id is not a control": func() (Key, error) {
			return d.Observation(testSSP, src, "ac-2_smt.a", testComponent, "2026-Q3")
		},
		"source uuid is not a uuid": func() (Key, error) {
			return d.Observation(testSSP, Source{UUID: "catalog-1", Vocabulary: canonical.VocabNIST80053}, "cp-4", testComponent, "2026-Q3")
		},
		"decision risk is not a uuid": func() (Key, error) {
			return d.AODecision(testSSP, "risk-4", "2026-10-11")
		},
		"decision ssp is not a uuid": func() (Key, error) {
			return d.AODecision("boundary-7", testComponent, "2026-10-11")
		},
		"decision period is malformed": func() (Key, error) {
			return d.AODecision(testSSP, testComponent, "11-10-2026")
		},
		"decision keyed on a quarter rather than a day": func() (Key, error) {
			return d.AODecision(testSSP, testComponent, "2026-Q4")
		},
		"responsibility half is invented": func() (Key, error) {
			return d.Responsibility(testSSP, src, "ia-2", testComponent, Half("owner"))
		},
		"responsibility ssp is not a uuid": func() (Key, error) {
			return d.Responsibility("boundary-7", src, "ia-2", testComponent, HalfProvider)
		},
		"responsibility component is not a uuid": func() (Key, error) {
			return d.Responsibility(testSSP, src, "ia-2", "web-01", HalfProvider)
		},
		"responsibility control is not a control": func() (Key, error) {
			return d.Responsibility(testSSP, src, "ac-2_smt.a", testComponent, HalfProvider)
		},
		"cell node is not a uuid": func() (Key, error) {
			return d.CellForControl("org-2", src, "cp-4", BucketToday)
		},
		"cell bucket is invented": func() (Key, error) {
			return d.CellForControl(testSSP, src, "cp-4", Bucket("+45"))
		},
		"cell family is not a family": func() (Key, error) {
			return d.CellForFamily(testSSP, src, "cp-4", BucketToday)
		},
		"evidence ssp is not a uuid": func() (Key, error) {
			return d.EvidenceResource("boundary-7", strings.Repeat("a", 64))
		},
	}
	for name, fn := range cases {
		if k, err := fn(); err == nil {
			t.Errorf("%s: derived %s instead of failing", name, k.UUID)
		}
	}
}

func TestResponsibilityHalvesAndCellAxesDoNotCollide(t *testing.T) {
	must := keyer(t)
	d := New(Namespace())
	src := nist()

	provider := must(d.Responsibility(testSSP, src, "ia-2", testComponent, HalfProvider))
	consumer := must(d.Responsibility(testSSP, src, "ia-2", testComponent, HalfConsumer))
	if provider.UUID == consumer.UUID {
		t.Error("the two halves of an inherited control share an identifier")
	}

	control := must(d.CellForControl(testSSP, src, "cp-4", BucketPlus30))
	family := must(d.CellForFamily(testSSP, src, "CP", BucketPlus30))
	if control.UUID == family.UUID {
		t.Error("a control cell and a family cell share an identifier")
	}
	if got := family.Fields[4]; got != "cp" {
		t.Errorf("family field = %q, want the canonical %q", got, "cp")
	}
}

func TestBuckets(t *testing.T) {
	for _, b := range []Bucket{BucketToday, BucketPlus7, BucketPlus14, BucketPlus30} {
		if _, err := b.canonical(); err != nil {
			t.Errorf("bucket %q rejected: %v", b, err)
		}
	}
	got, err := BucketOnDate("2026-10-11")
	if err != nil {
		t.Fatalf("BucketOnDate: %v", err)
	}
	if got != Bucket("2026-10-11") {
		t.Errorf("BucketOnDate = %q", got)
	}
	for _, bad := range []string{"2026-10", "2026-Q4", "11-10-2026", "tomorrow", ""} {
		if _, err := BucketOnDate(bad); err == nil {
			t.Errorf("BucketOnDate accepted %q", bad)
		}
	}

	// A calendar-impossible date is accepted, because SPARC's shared contract
	// types a decision date as ^\d{4}-\d{2}-\d{2}$ and nothing more. Being
	// stricter here would mean refusing to ingest an object a peer derived an
	// identifier for, which is the failure this grammar exists to prevent —
	// so the looseness is conformance, not an oversight. Raised upstream in
	// docs/dev/sparc-family-id-normalisation.md.
	for _, impossible := range []string{"2026-13-01", "2026-02-31", "2026-00-00"} {
		if _, err := BucketOnDate(impossible); err != nil {
			t.Errorf("BucketOnDate rejected %q; the shared contract accepts it", impossible)
		}
	}
}
