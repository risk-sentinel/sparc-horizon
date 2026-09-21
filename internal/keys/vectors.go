package keys

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/risk-sentinel/sparc-horizon/internal/canonical"
)

// Test vectors for the key grammar, published so the Ruby and Python ports in
// `sparc` (sparc#1161) are written against something other than prose.
//
// Each vector carries the arguments as a caller would issue them, the
// canonical field list they normalise to, and the resulting UUID. A port that
// disagrees on the UUID can read the field list and see which field it
// normalised differently, which is the difference between "we diverge" and a
// located bug.

// VectorDocument is the published vector file.
type VectorDocument struct {
	Grammar    string          `json:"grammar"`
	Namespace  VectorNamespace `json:"namespace"`
	Separator  string          `json:"separator"`
	About      []string        `json:"about"`
	Vectors    []Vector        `json:"vectors"`
	Assertions []Assertion     `json:"assertions"`
	JoinCases  []JoinCase      `json:"join-cases"`
	Rejections []Rejection     `json:"rejections"`
}

// VectorNamespace records which namespace the vectors were derived under, and
// how it was reached. When sparc#1155 registers the real one this block is the
// diff that shows every UUID below changed for a reason.
type VectorNamespace struct {
	UUID        string `json:"uuid"`
	Derivation  string `json:"derivation"`
	Provisional bool   `json:"provisional"`
	Issue       string `json:"issue"`
}

// Args is one vector's input, named as the field lists name them. Only the
// fields an object kind takes are populated.
type Args struct {
	ParentSSPUUID string `json:"parent-ssp-uuid,omitempty"`
	NodeUUID      string `json:"node-uuid,omitempty"`
	SourceUUID    string `json:"source-uuid,omitempty"`
	Vocabulary    string `json:"vocabulary,omitempty"`
	ControlID     string `json:"control-id,omitempty"`
	FamilyID      string `json:"family-id,omitempty"`
	ComponentUUID string `json:"component-uuid,omitempty"`
	Period        string `json:"period,omitempty"`
	RiskUUID      string `json:"risk-uuid,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	Half          string `json:"half,omitempty"`
	HorizonBucket string `json:"horizon-bucket,omitempty"`
}

// Vector is one derivation a port must reproduce exactly.
type Vector struct {
	Name            string   `json:"name"`
	Kind            string   `json:"kind"`
	Args            Args     `json:"args"`
	CanonicalFields []string `json:"canonical-fields"`
	UUID            string   `json:"uuid"`
}

// Assertion states a relation between vectors that must hold however they are
// computed. "same" means the UUIDs must be identical; "distinct" means no two
// may be equal.
type Assertion struct {
	Name     string   `json:"name"`
	Relation string   `json:"relation"`
	Vectors  []string `json:"vectors"`
	Why      string   `json:"why"`
}

// JoinCase tests the separator alone, below the field lists. These are not
// object keys: they exist because the grammar's predecessor joined on "|",
// under which ("a|b","c") and ("a","b|c") produced one input and therefore one
// UUID.
type JoinCase struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
	UUID   string   `json:"uuid"`
}

// Rejection is an input a conforming implementation must refuse. A port that
// derives a UUID here has a defect that no equality check would find.
type Rejection struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Args Args   `json:"args"`
	Why  string `json:"why"`
}

// Stable inputs for the vectors. The SSP, catalog, control and component
// values are the ones in the worked example in docs/03-data-model.md, so the
// document and the file agree by construction.
const (
	vecSSP       = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	vecSSPPeer   = "8c2d1f07-6a3b-4e15-9d8c-70b4e2a91f63"
	vecNISTCat   = "b7e21d90-4c1a-4f55-9e33-0a6d2c118f44"
	vecProfile   = "1e7c5a38-0d92-4b64-a7f1-52c8e3901b47"
	vecSecHub    = "5d40a72c-3e18-4f9b-86d2-0c7a41b5e926"
	vecComponent = "9f1c0f4e-2b7a-4d61-8f52-1c9a3b7d4e60"
	vecNode      = "2b0b4a6c-7d31-4e08-95af-6c1e8b204d7a"
	vecRisk      = "c1a2b3d4-5e6f-4071-8293-a4b5c6d7e8f9"
	vecPeriod    = "2026-Q3"
	// SHA-256 of the empty string, so a port can confirm it is hashing what
	// it thinks it is hashing.
	vecSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func nistSource() Source { return Source{UUID: vecNISTCat, Vocabulary: canonical.VocabNIST80053} }
func profileSource() Source {
	return Source{UUID: vecProfile, Vocabulary: canonical.VocabNIST80053}
}
func secHubSource() Source { return Source{UUID: vecSecHub, Vocabulary: canonical.VocabOpaque} }

// Vectors builds the published vector document for the given namespace.
func Vectors(ns uuid.UUID) (VectorDocument, error) {
	d := New(ns)
	doc := VectorDocument{
		Grammar:   GrammarVersion,
		Separator: canonical.Separator,
		Namespace: VectorNamespace{
			UUID:        ns.String(),
			Derivation:  fmt.Sprintf("uuidv5(urn:uuid:%s, %q)", uuid.NameSpaceURL, PlaceholderNamespaceURI),
			Provisional: ns == ProvisionalNamespace(),
			Issue:       "sparc#1155 — the federation namespace UUID is not registered yet",
		},
		About: []string{
			"Test vectors for the UUIDv5 key grammar specified in docs/03-data-model.md.",
			"input = grammar-version + separator + fields joined by the separator; uuid = uuidv5(namespace, input).",
			"canonical-fields is that joined list, already normalised, beginning with the grammar version.",
			"Every UUID here is provisional until the federation namespace UUID is registered.",
		},
	}

	type spec struct {
		name string
		kind Kind
		args Args
		fn   func() (Key, error)
	}

	controlArgs := func(src Source, controlID, period string) Args {
		return Args{
			ParentSSPUUID: vecSSP,
			SourceUUID:    src.UUID,
			Vocabulary:    src.Vocabulary.String(),
			ControlID:     controlID,
			ComponentUUID: vecComponent,
			Period:        period,
		}
	}
	cellArgs := func(src Source, controlID, familyID string, bucket Bucket) Args {
		return Args{
			NodeUUID:      vecNode,
			SourceUUID:    src.UUID,
			Vocabulary:    src.Vocabulary.String(),
			ControlID:     controlID,
			FamilyID:      familyID,
			HorizonBucket: string(bucket),
		}
	}

	nist, profile, secHub := nistSource(), profileSource(), secHubSource()

	decision, err := BucketOnDate("2026-10-11")
	if err != nil {
		return VectorDocument{}, err
	}

	specs := []spec{
		// One object of each control-scoped kind on identical arguments, so
		// the object-kind token is visibly the only thing separating them.
		{"attestation-worked-example", KindAttestation, controlArgs(nist, "cp-4", vecPeriod), func() (Key, error) {
			return d.Attestation(vecSSP, nist, "cp-4", vecComponent, vecPeriod)
		}},
		{"observation-same-arguments", KindObservation, controlArgs(nist, "cp-4", vecPeriod), func() (Key, error) {
			return d.Observation(vecSSP, nist, "cp-4", vecComponent, vecPeriod)
		}},
		{"finding-same-arguments", KindFinding, controlArgs(nist, "cp-4", vecPeriod), func() (Key, error) {
			return d.Finding(vecSSP, nist, "cp-4", vecComponent, vecPeriod)
		}},
		{"risk-same-arguments", KindRisk, controlArgs(nist, "cp-4", vecPeriod), func() (Key, error) {
			return d.Risk(vecSSP, nist, "cp-4", vecComponent, vecPeriod)
		}},
		{"poam-item-same-arguments", KindPOAMItem, controlArgs(nist, "cp-4", vecPeriod), func() (Key, error) {
			return d.POAMItem(vecSSP, nist, "cp-4", vecComponent, vecPeriod)
		}},

		// The spellings SPARC's own API has been observed to emit (sparc#1162).
		{"attestation-nist-spelling-1", KindAttestation, controlArgs(nist, "CP-4", vecPeriod), func() (Key, error) {
			return d.Attestation(vecSSP, nist, "CP-4", vecComponent, vecPeriod)
		}},
		{"attestation-nist-spelling-2", KindAttestation, controlArgs(nist, "cp-04", vecPeriod), func() (Key, error) {
			return d.Attestation(vecSSP, nist, "cp-04", vecComponent, vecPeriod)
		}},
		{"attestation-nist-spelling-3", KindAttestation, controlArgs(nist, "Cp-4", vecPeriod), func() (Key, error) {
			return d.Attestation(vecSSP, nist, "Cp-4", vecComponent, vecPeriod)
		}},

		{"observation-enhancement-spelling-1", KindObservation, controlArgs(nist, "AC-2(1)", vecPeriod), func() (Key, error) {
			return d.Observation(vecSSP, nist, "AC-2(1)", vecComponent, vecPeriod)
		}},
		{"observation-enhancement-spelling-2", KindObservation, controlArgs(nist, "AC-2 (1)", vecPeriod), func() (Key, error) {
			return d.Observation(vecSSP, nist, "AC-2 (1)", vecComponent, vecPeriod)
		}},
		{"observation-enhancement-spelling-3", KindObservation, controlArgs(nist, "AC-2.1", vecPeriod), func() (Key, error) {
			return d.Observation(vecSSP, nist, "AC-2.1", vecComponent, vecPeriod)
		}},
		{"observation-enhancement-spelling-4", KindObservation, controlArgs(nist, "ac-2.1", vecPeriod), func() (Key, error) {
			return d.Observation(vecSSP, nist, "ac-2.1", vecComponent, vecPeriod)
		}},
		// The enhancement is a control in its own right, not a spelling of
		// its parent.
		{"observation-parent-control", KindObservation, controlArgs(nist, "ac-2", vecPeriod), func() (Key, error) {
			return d.Observation(vecSSP, nist, "ac-2", vecComponent, vecPeriod)
		}},

		// The qualifier at work: one component, one period, one SSP,
		// assessed against two authorities.
		{"observation-security-hub", KindObservation, controlArgs(secHub, "ACM.1", vecPeriod), func() (Key, error) {
			return d.Observation(vecSSP, secHub, "ACM.1", vecComponent, vecPeriod)
		}},
		// "acm.1" is a legal opaque identifier and a different one. This is
		// the pair that makes lowercasing another authority's identifier
		// visible as a data error rather than a formatting preference.
		{"observation-security-hub-lowercased", KindObservation, controlArgs(secHub, "acm.1", vecPeriod), func() (Key, error) {
			return d.Observation(vecSSP, secHub, "acm.1", vecComponent, vecPeriod)
		}},
		// The same NIST control resolved through the profile rather than the
		// catalog is a different key: the resolving document differs.
		{"observation-resolved-through-profile", KindObservation, controlArgs(profile, "cp-4", vecPeriod), func() (Key, error) {
			return d.Observation(vecSSP, profile, "cp-4", vecComponent, vecPeriod)
		}},
		{"observation-next-period", KindObservation, controlArgs(nist, "cp-4", "2026-Q4"), func() (Key, error) {
			return d.Observation(vecSSP, nist, "cp-4", vecComponent, "2026-Q4")
		}},
		{"observation-peer-boundary", KindObservation, func() Args {
			a := controlArgs(nist, "cp-4", vecPeriod)
			a.ParentSSPUUID = vecSSPPeer
			return a
		}(), func() (Key, error) {
			return d.Observation(vecSSPPeer, nist, "cp-4", vecComponent, vecPeriod)
		}},

		// Evidence resources are keyed on content, take no source-uuid, and
		// normalise the digest's case.
		{"evidence-resource", KindResource, Args{ParentSSPUUID: vecSSP, SHA256: vecSHA256}, func() (Key, error) {
			return d.EvidenceResource(vecSSP, vecSHA256)
		}},
		{"evidence-resource-uppercase-hash", KindResource, Args{ParentSSPUUID: vecSSP, SHA256: strings.ToUpper(vecSHA256)}, func() (Key, error) {
			return d.EvidenceResource(vecSSP, strings.ToUpper(vecSHA256))
		}},

		{"ao-decision", KindDecision, Args{ParentSSPUUID: vecSSP, RiskUUID: vecRisk, Period: "2026-10-11"}, func() (Key, error) {
			return d.AODecision(vecSSP, vecRisk, "2026-10-11")
		}},

		// The two halves of an inherited control.
		{"responsibility-provider", KindResponsibility, func() Args {
			a := controlArgs(nist, "ia-2", "")
			a.Half = string(HalfProvider)
			return a
		}(), func() (Key, error) {
			return d.Responsibility(vecSSP, nist, "ia-2", vecComponent, HalfProvider)
		}},
		{"responsibility-consumer", KindResponsibility, func() Args {
			a := controlArgs(nist, "ia-2", "")
			a.Half = string(HalfConsumer)
			return a
		}(), func() (Key, error) {
			return d.Responsibility(vecSSP, nist, "ia-2", vecComponent, HalfConsumer)
		}},

		// Projection cells: a control column, a family column, and a
		// decision-date bucket.
		{"cell-for-control", KindCell, cellArgs(nist, "cp-4", "", BucketPlus30), func() (Key, error) {
			return d.CellForControl(vecNode, nist, "cp-4", BucketPlus30)
		}},
		{"cell-for-family", KindCell, cellArgs(nist, "", "CP", BucketPlus30), func() (Key, error) {
			return d.CellForFamily(vecNode, nist, "CP", BucketPlus30)
		}},
		{"cell-on-decision-date", KindCell, cellArgs(nist, "", "cp", decision), func() (Key, error) {
			return d.CellForFamily(vecNode, nist, "cp", decision)
		}},
	}

	for _, sp := range specs {
		k, err := sp.fn()
		if err != nil {
			return VectorDocument{}, fmt.Errorf("vector %s: %w", sp.name, err)
		}
		doc.Vectors = append(doc.Vectors, Vector{
			Name:            sp.name,
			Kind:            string(sp.kind),
			Args:            sp.args,
			CanonicalFields: k.Fields,
			UUID:            k.UUID.String(),
		})
	}

	doc.Assertions = []Assertion{
		{
			Name:     "object-kind-separates-otherwise-identical-keys",
			Relation: "distinct",
			Vectors:  []string{"attestation-worked-example", "observation-same-arguments", "finding-same-arguments", "risk-same-arguments", "poam-item-same-arguments"},
			Why:      "The five kinds share a field list; only the object-kind token distinguishes them.",
		},
		{
			Name:     "nist-spellings-converge",
			Relation: "same",
			Vectors:  []string{"attestation-worked-example", "attestation-nist-spelling-1", "attestation-nist-spelling-2", "attestation-nist-spelling-3"},
			Why:      "SPARC's API emits control identifiers in more than one casing (sparc#1162); all of them name one control.",
		},
		{
			Name:     "enhancement-spellings-converge",
			Relation: "same",
			Vectors:  []string{"observation-enhancement-spelling-1", "observation-enhancement-spelling-2", "observation-enhancement-spelling-3", "observation-enhancement-spelling-4"},
			Why:      "Parenthesised, spaced and dotted enhancements are one control; OSCAL's form is the dotted one.",
		},
		{
			Name:     "enhancement-is-not-its-parent",
			Relation: "distinct",
			Vectors:  []string{"observation-enhancement-spelling-4", "observation-parent-control"},
			Why:      "ac-2.1 is a control in its own right.",
		},
		{
			Name:     "catalog-authority-partitions-the-key-space",
			Relation: "distinct",
			Vectors:  []string{"observation-same-arguments", "observation-security-hub", "observation-resolved-through-profile"},
			Why:      "The same component, period and SSP assessed against two authorities must not collide; nor must one authority's identifier resolved through two documents.",
		},
		{
			Name:     "opaque-identifiers-are-case-sensitive",
			Relation: "distinct",
			Vectors:  []string{"observation-security-hub", "observation-security-hub-lowercased"},
			Why:      "ACM.1 is an AWS Security Hub control; acm.1 names nothing. An implementation that lowercases them into one has lost the difference.",
		},
		{
			Name:     "period-and-boundary-are-key-fields",
			Relation: "distinct",
			Vectors:  []string{"observation-same-arguments", "observation-next-period", "observation-peer-boundary"},
			Why:      "A later period and a peer boundary are different objects, not revisions of one.",
		},
		{
			Name:     "hash-case-is-normalised",
			Relation: "same",
			Vectors:  []string{"evidence-resource", "evidence-resource-uppercase-hash"},
			Why:      "The same bytes submitted twice are the same resource however the digest was spelled.",
		},
		{
			Name:     "responsibility-halves-do-not-collide",
			Relation: "distinct",
			Vectors:  []string{"responsibility-provider", "responsibility-consumer"},
			Why:      "A hybrid control is green only when both halves hold; one identifier for both would make that uncheckable.",
		},
		{
			Name:     "control-and-family-columns-are-distinct-cells",
			Relation: "distinct",
			Vectors:  []string{"cell-for-control", "cell-for-family", "cell-on-decision-date"},
			Why:      "A family rollup is not the control it contains, and a decision-date bucket is not the +30 bucket.",
		},
	}

	// The separator, tested below the field lists.
	for _, c := range []struct {
		name   string
		fields []string
	}{
		{"join-ambiguity-left", []string{"a|b", "c"}},
		{"join-ambiguity-right", []string{"a", "b|c"}},
	} {
		doc.JoinCases = append(doc.JoinCases, JoinCase{
			Name:   c.name,
			Fields: c.fields,
			UUID:   uuid.NewSHA1(ns, []byte(strings.Join(c.fields, canonical.Separator))).String(),
		})
	}

	doc.Rejections = []Rejection{
		{
			Name: "no-source-uuid-resolved",
			Kind: string(KindAttestation),
			Args: Args{ParentSSPUUID: vecSSP, Vocabulary: canonical.VocabNIST80053.String(), ControlID: "cp-4", ComponentUUID: vecComponent, Period: vecPeriod},
			Why:  "A key missing a field is a different key, not a key with an empty field. Reject rather than derive.",
		},
		{
			Name: "no-vocabulary-resolved",
			Kind: string(KindAttestation),
			Args: Args{ParentSSPUUID: vecSSP, SourceUUID: vecNISTCat, ControlID: "cp-4", ComponentUUID: vecComponent, Period: vecPeriod},
			Why:  "Without a vocabulary there is no way to know whether the identifier may be canonicalised.",
		},
		{
			Name: "statement-fragment-is-not-a-control",
			Kind: string(KindObservation),
			Args: Args{ParentSSPUUID: vecSSP, SourceUUID: vecNISTCat, Vocabulary: canonical.VocabNIST80053.String(), ControlID: "ac-2_smt.a", ComponentUUID: vecComponent, Period: vecPeriod},
			Why:  "It names part of a control; keying on it counts one object twice.",
		},
		{
			Name: "security-hub-identifier-under-the-nist-vocabulary",
			Kind: string(KindObservation),
			Args: Args{ParentSSPUUID: vecSSP, SourceUUID: vecSecHub, Vocabulary: canonical.VocabNIST80053.String(), ControlID: "ACM.1", ComponentUUID: vecComponent, Period: vecPeriod},
			Why:  "Canonicalising it would yield acm.1, which validates and names nothing.",
		},
		{
			Name: "period-not-zero-padded",
			Kind: string(KindObservation),
			Args: Args{ParentSSPUUID: vecSSP, SourceUUID: vecNISTCat, Vocabulary: canonical.VocabNIST80053.String(), ControlID: "cp-4", ComponentUUID: vecComponent, Period: "2026-1"},
			Why:  "2026-1 is not a spelling of 2026-01; it is a rejection.",
		},
		{
			Name: "quarter-without-a-hyphen",
			Kind: string(KindObservation),
			Args: Args{ParentSSPUUID: vecSSP, SourceUUID: vecNISTCat, Vocabulary: canonical.VocabNIST80053.String(), ControlID: "cp-4", ComponentUUID: vecComponent, Period: "2026Q3"},
			Why:  "Quarters are 2026-Q3, never 2026Q3 or Q3-2026.",
		},
		{
			Name: "component-is-not-a-uuid",
			Kind: string(KindObservation),
			Args: Args{ParentSSPUUID: vecSSP, SourceUUID: vecNISTCat, Vocabulary: canonical.VocabNIST80053.String(), ControlID: "cp-4", ComponentUUID: "web-01", Period: vecPeriod},
			Why:  "Components join by UUID; a local name would not federate.",
		},
		{
			Name: "separator-inside-a-field",
			Kind: string(KindObservation),
			Args: Args{ParentSSPUUID: vecSSP, SourceUUID: vecSecHub, Vocabulary: canonical.VocabOpaque.String(), ControlID: "ACM\u001f1", ComponentUUID: vecComponent, Period: vecPeriod},
			Why:  "Rejected rather than escaped: an escape sequence is itself a value a field could contain.",
		},
		{
			Name: "hash-that-is-not-sha-256",
			Kind: string(KindResource),
			Args: Args{ParentSSPUUID: vecSSP, SHA256: "deadbeef"},
			Why:  "An evidence resource keyed on anything but the digest of its content is not this object.",
		},
		{
			Name: "responsibility-half-outside-the-vocabulary",
			Kind: string(KindResponsibility),
			Args: Args{ParentSSPUUID: vecSSP, SourceUUID: vecNISTCat, Vocabulary: canonical.VocabNIST80053.String(), ControlID: "ia-2", ComponentUUID: vecComponent, Half: "owner"},
			Why:  "There are two halves, and a third would silently create a third object.",
		},
		{
			Name: "horizon-bucket-outside-the-vocabulary",
			Kind: string(KindCell),
			Args: Args{NodeUUID: vecNode, SourceUUID: vecNISTCat, Vocabulary: canonical.VocabNIST80053.String(), ControlID: "cp-4", HorizonBucket: "+45"},
			Why:  "Only today, +7, +14, +30 and decision dates are materialised; a cell keyed elsewhere is never invalidated.",
		},
	}

	return doc, nil
}
