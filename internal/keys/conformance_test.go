package keys

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"

	"github.com/risk-sentinel/sparc-horizon/internal/canonical"
)

// Conformance against SPARC's shared key-grammar contract.
//
// sparc#1161 made SPARC the source of truth for the grammar, its field lists
// and the vectors all three runtimes assert against. Horizon consumes that
// file rather than publishing its own, so this is the test that says the Go
// reference still agrees with the Ruby and Python ones.
//
// It deliberately checks more than the vectors' UUIDs. Every vector agreed on
// the day this was written, and reading the contract's *type rules* still
// found five disagreements — a period form, a decision's date form, a control
// id's prefix length and enhancement depth, a family id's length, and a
// normalisation that derives two different UUIDs for one input. UUID agreement
// over a fixed corpus is a weak check; it only exercises what somebody thought
// to write a vector for.

const (
	contractFile   = "testdata/key-grammar.v1.json"
	provenanceFile = "testdata/PROVENANCE.json"
)

// rulePattern lifts the anchored expression out of a type rule that states a
// regex and then explains it in prose.
var rulePattern = regexp.MustCompile(`\^[^ ]*\$`)

type contract struct {
	Grammar   string `json:"grammar"`
	Separator string `json:"separator"`
	Namespace struct {
		UUID        string `json:"uuid"`
		Derivation  string `json:"derivation"`
		Provisional bool   `json:"provisional"`
	} `json:"namespace"`
	Types      map[string]string      `json:"types"`
	FieldLists map[string][]fieldSpec `json:"field-lists"`
	Vectors    []contractVector       `json:"vectors"`
	JoinCases  []struct {
		Name   string   `json:"name"`
		Fields []string `json:"fields"`
		UUID   string   `json:"uuid"`
	} `json:"join-cases"`
	Rejections []struct {
		Name string       `json:"name"`
		Kind string       `json:"kind"`
		Args contractArgs `json:"args"`
		Why  string       `json:"why"`
	} `json:"rejections"`
	Dedup struct {
		Key      []string `json:"key"`
		Conflict string   `json:"conflict"`
	} `json:"dedup"`
}

type fieldSpec struct {
	Arg     string   `json:"arg"`
	Literal string   `json:"literal"`
	OneOf   []string `json:"one-of"`
}

type contractArgs struct {
	ParentSSPUUID string `json:"parent-ssp-uuid"`
	NodeUUID      string `json:"node-uuid"`
	SourceUUID    string `json:"source-uuid"`
	Vocabulary    string `json:"vocabulary"`
	ControlID     string `json:"control-id"`
	FamilyID      string `json:"family-id"`
	ComponentUUID string `json:"component-uuid"`
	Period        string `json:"period"`
	RiskUUID      string `json:"risk-uuid"`
	SHA256        string `json:"sha256"`
	Half          string `json:"half"`
	HorizonBucket string `json:"horizon-bucket"`
}

type contractVector struct {
	Name            string       `json:"name"`
	Kind            string       `json:"kind"`
	Args            contractArgs `json:"args"`
	CanonicalFields []string     `json:"canonical-fields"`
	UUID            string       `json:"uuid"`
}

func loadContract(t *testing.T) contract {
	t.Helper()

	b, err := os.ReadFile(contractFile)
	if err != nil {
		t.Fatalf("reading the contract: %v", err)
	}
	var c contract
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("contract: %v", err)
	}
	return c
}

// deriveFromContract is the recipe a port follows to consume the file, and the
// one this package is measured by.
func deriveFromContract(d Deriver, kind string, a contractArgs) (Key, error) {
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

func TestVendoredContractMatchesItsProvenance(t *testing.T) {
	b, err := os.ReadFile(provenanceFile)
	if err != nil {
		t.Fatalf("reading provenance: %v", err)
	}
	var p struct {
		Upstream struct {
			Repository string `json:"repository"`
			Path       string `json:"path"`
			Commit     string `json:"commit"`
			Retrieved  string `json:"retrieved"`
		} `json:"upstream"`
		File struct {
			SHA256 string `json:"sha256"`
			Bytes  int    `json:"bytes"`
		} `json:"file"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("provenance: %v", err)
	}
	if p.Upstream.Repository == "" || p.Upstream.Commit == "" || p.Upstream.Retrieved == "" {
		t.Error("the manifest does not say where the contract came from or when")
	}

	got, err := os.ReadFile(contractFile)
	if err != nil {
		t.Fatalf("reading the contract: %v", err)
	}
	sum := sha256.Sum256(got)
	if h := hex.EncodeToString(sum[:]); h != p.File.SHA256 {
		t.Errorf("contract digest %s, manifest says %s", h, p.File.SHA256)
	}
	if len(got) != p.File.Bytes {
		t.Errorf("contract is %d bytes, manifest says %d", len(got), p.File.Bytes)
	}
}

// The registration itself: the namespace is derived from the published URI, so
// it is recomputed here rather than compared to a constant somebody copied.
func TestRegisteredNamespaceAgrees(t *testing.T) {
	c := loadContract(t)

	if c.Namespace.UUID != Namespace().String() {
		t.Errorf("contract namespace %s, ours %s", c.Namespace.UUID, Namespace())
	}
	if want := uuid.NewSHA1(uuid.NameSpaceURL, []byte(NamespaceURI)); want != Namespace() {
		t.Errorf("Namespace() is not uuidv5(url, %q)", NamespaceURI)
	}
	if c.Namespace.Provisional {
		t.Error("the contract still calls the namespace provisional")
	}
	if c.Grammar != GrammarVersion {
		t.Errorf("contract grammar %q, ours %q", c.Grammar, GrammarVersion)
	}
	if c.Separator != canonical.Separator {
		t.Errorf("contract separator %q, ours %q", c.Separator, canonical.Separator)
	}
}

// Every vector reproduces, in UUID and in canonical field list.
func TestContractVectorsReproduce(t *testing.T) {
	c := loadContract(t)
	d := New(Namespace())

	if len(c.Vectors) == 0 {
		t.Fatal("the contract carries no vectors")
	}
	reproduced := 0
	for _, v := range c.Vectors {
		k, err := deriveFromContract(d, v.Kind, v.Args)
		if err != nil {
			t.Errorf("%s: %v", v.Name, err)
			continue
		}
		if k.UUID.String() != v.UUID {
			t.Errorf("%s: derived %s, contract says %s", v.Name, k.UUID, v.UUID)
			continue
		}
		if len(k.Fields) != len(v.CanonicalFields) {
			t.Errorf("%s: %d canonical fields, contract says %d", v.Name, len(k.Fields), len(v.CanonicalFields))
			continue
		}
		for i := range k.Fields {
			if k.Fields[i] != v.CanonicalFields[i] {
				t.Errorf("%s: field %d is %q, contract says %q", v.Name, i, k.Fields[i], v.CanonicalFields[i])
			}
		}
		reproduced++
	}
	if reproduced != len(c.Vectors) {
		t.Errorf("reproduced %d of %d vectors", reproduced, len(c.Vectors))
	}
}

// The field lists themselves, not only the values they produce. A vector set
// that thinned out would stop covering a kind without anything saying so.
func TestFieldListsMatchTheContract(t *testing.T) {
	c := loadContract(t)
	d := New(Namespace())

	byKind := map[string]contractVector{}
	for _, v := range c.Vectors {
		byKind[v.Kind] = v
	}
	if len(c.FieldLists) != 9 {
		t.Errorf("the contract carries %d field lists, want 9", len(c.FieldLists))
	}

	for kind, spec := range c.FieldLists {
		v, ok := byKind[kind]
		if !ok {
			t.Errorf("%s: the contract defines a field list with no vector to check it against", kind)
			continue
		}
		k, err := deriveFromContract(d, kind, v.Args)
		if err != nil {
			t.Errorf("%s: %v", kind, err)
			continue
		}
		// Field 0 is the grammar version; the contract's list starts after it.
		if len(k.Fields)-1 != len(spec) {
			t.Errorf("%s: we hash %d fields after the grammar version, the contract lists %d", kind, len(k.Fields)-1, len(spec))
			continue
		}
		for i, f := range spec {
			got := k.Fields[i+1]
			if f.Literal != "" && got != f.Literal {
				t.Errorf("%s: field %d is %q, the contract's literal is %q", kind, i, got, f.Literal)
			}
			if f.Arg == "" && f.Literal == "" && len(f.OneOf) == 0 {
				t.Errorf("%s: field %d in the contract is neither an arg, a literal nor a one-of", kind, i)
			}
		}
	}
}

// The type rules. This is the check that was missing: every divergence found
// while adopting the contract was a type rule, and not one of them changed a
// vector.
func TestTypeRulesAgree(t *testing.T) {
	c := loadContract(t)

	// The contract states most types as a regex followed by prose explaining
	// it — "^[a-z]{2,3}$ after lowercasing." — so the pattern is extracted
	// rather than compiled whole. Extracting beats copying: a rule that moves
	// upstream fails here instead of being silently stale.
	mustRe := func(name string) *regexp.Regexp {
		t.Helper()
		rule, ok := c.Types[name]
		if !ok {
			t.Fatalf("the contract has no type %q", name)
		}
		pattern := rulePattern.FindString(rule)
		if pattern == "" {
			t.Fatalf("type %q states no ^...$ pattern this test can drive: %q", name, rule)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			t.Fatalf("type %q pattern %q does not compile: %v", name, pattern, err)
		}
		return re
	}

	t.Run("period", func(t *testing.T) {
		re := mustRe("period")
		for _, v := range []string{"2026-Q1", "2026-Q4", "2026-01", "2026-12", "2026", "2026-09-21", "2026-Q5", "2026-1", "2026Q3", "", "Q3-2026"} {
			_, err := canonical.Period(v)
			if got, want := err == nil, re.MatchString(v); got != want {
				t.Errorf("period %q: we accept=%v, the contract accepts=%v", v, got, want)
			}
		}
	})

	t.Run("decision-date", func(t *testing.T) {
		re := mustRe("decision-date")
		for _, v := range []string{"2026-10-11", "2026-01-01", "2026-Q4", "2026-10", "2026", "11-10-2026", ""} {
			_, err := canonical.DecisionDate(v)
			if got, want := err == nil, re.MatchString(v); got != want {
				t.Errorf("decision date %q: we accept=%v, the contract accepts=%v", v, got, want)
			}
		}
	})

	t.Run("family-id under the NIST vocabulary", func(t *testing.T) {
		re := mustRe("family-id")
		for _, v := range []string{"ac", "AC", "sar", "SAR", "a", "abcd", "", "a1"} {
			got, err := canonical.FamilyID(canonical.VocabNIST80053, v)
			// The contract's rule applies to the value after lowercasing.
			want := re.MatchString(lower(v))
			if (err == nil) != want {
				t.Errorf("family %q: we accept=%v, the contract accepts=%v", v, err == nil, want)
			}
			if err == nil && !re.MatchString(got) {
				t.Errorf("family %q canonicalised to %q, which the contract's own rule rejects", v, got)
			}
		}
	})

	t.Run("control-id canonical form", func(t *testing.T) {
		// The contract states the canonicalised value must match this. It is
		// prose around a regex, so the regex is spelled here and the contract's
		// text asserted to still contain it.
		const canonicalForm = `^[a-z]{2,3}-\d+(\.\d+)*$`
		if rule := c.Types["control-id"]; !contains(rule, canonicalForm) {
			t.Fatalf("the contract's control-id rule no longer states %s: %q", canonicalForm, rule)
		}
		re := regexp.MustCompile(canonicalForm)
		for _, v := range []string{"AC-2", "ac-02", "AC-2(1)", "AC-2 (1)", "ac-2.1.3", "AC-2(1)(3)", "sar-5"} {
			got, err := canonical.ControlID(canonical.VocabNIST80053, v)
			if err != nil {
				t.Errorf("control %q: %v", v, err)
				continue
			}
			if !re.MatchString(got) {
				t.Errorf("control %q canonicalised to %q, which the contract's own rule rejects", v, got)
			}
		}
	})

	t.Run("sha256 and uuid", func(t *testing.T) {
		shaRe, uuidRe := mustRe("sha256"), mustRe("uuid")
		const digest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		if !shaRe.MatchString(digest) {
			t.Error("the contract rejects a SHA-256 digest this package accepts")
		}
		// The contract's rule applies after lowercasing, which the deriver
		// does before hashing.
		if shaRe.MatchString("E3B0C442") {
			t.Error("the contract's sha256 rule matched an uppercase stub; the pattern was extracted wrongly")
		}
		got, err := canonical.UUID("3FA85F64-5717-4562-B3FC-2C963F66AFA6")
		if err != nil || !uuidRe.MatchString(got) {
			t.Errorf("UUID canonicalised to %q (%v), which the contract's rule rejects", got, err)
		}
	})
}

// The one rule Horizon does not adopt, asserted as a disagreement so it cannot
// quietly resolve, widen, or be forgotten.
//
// The contract normalises `family-id` with `lowercase` unconditionally. Under
// an opaque vocabulary that is the defect #37 removed from control ids: an AWS
// Security Hub family is `ACM`, and `acm` names nothing. Horizon holds
// NFC-only there. The consequence is a genuine divergence — the same input
// derives two different identifiers, with neither side erroring — which is why
// it is written down in docs/dev/sparc-family-id-normalisation.md rather than
// absorbed.
func TestFamilyIDNormalisationDivergesFromTheContract(t *testing.T) {
	c := loadContract(t)

	spec, ok := c.FieldLists["cell"]
	if !ok {
		t.Fatal("the contract has no cell field list")
	}
	found := false
	for _, f := range spec {
		if len(f.OneOf) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("the cell field list no longer carries a one-of; the divergence may have moved")
	}

	d := New(Namespace())
	upper, err := d.CellForFamily("2b0b4a6c-7d31-4e08-95af-6c1e8b204d7a",
		Source{UUID: "5d40a72c-3e18-4f9b-86d2-0c7a41b5e926", Vocabulary: canonical.VocabOpaque},
		"ACM", BucketToday)
	if err != nil {
		t.Fatalf("opaque family ACM: %v", err)
	}
	lowerKey, err := d.CellForFamily("2b0b4a6c-7d31-4e08-95af-6c1e8b204d7a",
		Source{UUID: "5d40a72c-3e18-4f9b-86d2-0c7a41b5e926", Vocabulary: canonical.VocabOpaque},
		"acm", BucketToday)
	if err != nil {
		t.Fatalf("opaque family acm: %v", err)
	}

	if upper.UUID == lowerKey.UUID {
		t.Error("ACM and acm now derive one identifier under an opaque vocabulary — either the rule changed here, or the disagreement is resolved and docs/dev/sparc-family-id-normalisation.md is stale")
	}
	t.Logf("held divergence: opaque family ACM -> %s, acm -> %s; the contract would derive the second for both", upper.UUID, lowerKey.UUID)
}

func lower(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + 32
		}
	}
	return string(out)
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
