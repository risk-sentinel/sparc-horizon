// Package keys derives Horizon's object identifiers.
//
// It is the Go reference implementation of the UUIDv5 key grammar specified
// normatively in docs/03-data-model.md. Ruby and Python ports live in `sparc`
// (sparc#1161) and are written against the same test vectors this package
// emits, because three implementations of a hashing grammar disagree
// eventually and nothing else catches it: two peers deriving two identifiers
// for one object is a silent data-integrity fault, not a bug someone notices.
//
// Two properties are load-bearing and easy to lose in a refactor:
//
//   - The grammar version is the first field of every key. A change to any
//     field list changes every identifier derived under it, which is as
//     breaking as changing the namespace.
//   - There is no exported way to derive from raw strings. Every entry point
//     canonicalises its own fields and rejects what it cannot canonicalise,
//     so an identifier cannot be minted from a value another implementation
//     would have normalised differently.
//
// Every entry point returns a Key rather than a bare UUID. The canonical field
// list is what a port needs to see when two implementations disagree: without
// it a mismatch says only that they diverged, never where.
package keys

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/risk-sentinel/sparc-horizon/internal/canonical"
)

// GrammarVersion is v1 of docs/03-data-model.md § Deterministic UUIDs. It is
// hashed into every key, so old and new identifiers cannot be mistaken for
// each other when the grammar changes. A change to a field list is a v2 of
// that document, never an edit to v1.
const GrammarVersion = "v1"

// NamespaceURI is the registered SPARC namespace URI (sparc#1155, decided
// 2026-09-21). Horizon adopted SPARC's rather than the placeholder it carried:
// a namespace identifies an authority's vocabulary, so it has to be the same
// string in every deployment and in every runtime.
const NamespaceURI = "https://sparc.risk-sentinel.org/ns"

// Kind is the object-kind token that sits second in every key. The tokens are
// fixed lowercase ASCII from the field-list table, not free text.
type Kind string

// The nine object kinds. Five share a field list; the other four differ,
// which is why they are separate rows in the document rather than a flag.
const (
	KindAttestation    Kind = "attestation"
	KindObservation    Kind = "observation"
	KindFinding        Kind = "finding"
	KindRisk           Kind = "risk"
	KindPOAMItem       Kind = "poam-item"
	KindResource       Kind = "resource"
	KindDecision       Kind = "decision"
	KindResponsibility Kind = "responsibility"
	KindCell           Kind = "cell"
)

// Half names the two sides of an inherited control. A hybrid control carries
// both, and is green only when both have unexpired observations.
type Half string

const (
	HalfProvider Half = "provider"
	HalfConsumer Half = "consumer"
)

// Source qualifies a control identifier with the catalog or profile that
// defines it. A control identifier is unique only within its catalog: "ac-2.1"
// is NIST SP 800-53, "ACM.1" is AWS Security Hub, and one SSP can carry both
// because Security Hub identifiers arrive through inherited AWS service
// components.
//
// UUID is the UUID of the back-matter resource the `source` on the declaring
// control-implementation resolves to — never the document-local "#fragment",
// which would not federate. Where `source` is absent it is the SSP's
// `import-profile`, resolved the same way.
type Source struct {
	UUID       string
	Vocabulary canonical.Vocabulary
}

func (s Source) canonical() (string, error) {
	if s.UUID == "" {
		// A key missing a field is not a key with an empty field; it is a
		// different key. Deriving without a resolved source-uuid is the
		// silent divergence the qualifier exists to prevent.
		return "", fmt.Errorf("keys: no source-uuid resolved for the control; refusing to derive")
	}
	return canonical.UUID(s.UUID)
}

// Namespace is the registered federation namespace every object identity in
// the estate derives under: 9f434272-f796-589b-b972-954790395630.
//
// It is derived rather than invented — uuidv5 of the namespace URI under the
// standard URL namespace — so any peer recomputes it from the published URI
// instead of being told a constant it has to copy correctly, and can verify it
// provably belongs to that namespace. SPARC registered it the same way
// (sparc#1155), and `sparc:lib/federation/key-grammar.v1.json` publishes both
// the value and the derivation.
func Namespace() uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(NamespaceURI))
}

// Key is a derived identifier together with the exact canonical fields it was
// derived from, in order and beginning with the grammar version.
type Key struct {
	Kind   Kind
	Fields []string
	UUID   uuid.UUID
}

// Input returns the bytes hashed under the namespace: the fields joined with
// the unit separator.
func (k Key) Input() string { return strings.Join(k.Fields, canonical.Separator) }

// String returns the derived UUID in its canonical textual form.
func (k Key) String() string { return k.UUID.String() }

// Deriver derives identifiers under one federation namespace.
type Deriver struct {
	ns uuid.UUID
}

// New returns a Deriver over the given federation namespace. The namespace is
// always a parameter: a package that reads a provisional value implicitly is a
// package that keeps deriving under it after the real one is registered.
func New(ns uuid.UUID) Deriver { return Deriver{ns: ns} }

// Namespace reports the namespace this Deriver derives under.
func (d Deriver) Namespace() uuid.UUID { return d.ns }

// derive joins the grammar version and the given canonical fields with the
// unit separator and hashes the result under the namespace.
//
// NIST SP 800-53 SI-10: nothing reaches this function that has not been
// canonicalised and validated by its caller, and canonical.Join rejects a
// field carrying the separator even so.
func (d Deriver) derive(kind Kind, fields ...string) (Key, error) {
	all := append([]string{GrammarVersion}, fields...)
	input, err := canonical.Join(all...)
	if err != nil {
		return Key{}, err
	}
	return Key{Kind: kind, Fields: all, UUID: uuid.NewSHA1(d.ns, []byte(input))}, nil
}

// controlScoped derives the five object kinds that share a field list:
// parent-ssp-uuid, kind, source-uuid, control-id, component-uuid, period.
func (d Deriver) controlScoped(kind Kind, parentSSP string, src Source, controlID, componentUUID, period string) (Key, error) {
	ssp, err := canonical.UUID(parentSSP)
	if err != nil {
		return Key{}, fmt.Errorf("parent ssp: %w", err)
	}
	source, err := src.canonical()
	if err != nil {
		return Key{}, err
	}
	control, err := canonical.ControlID(src.Vocabulary, controlID)
	if err != nil {
		return Key{}, err
	}
	component, err := canonical.UUID(componentUUID)
	if err != nil {
		return Key{}, fmt.Errorf("component: %w", err)
	}
	p, err := canonical.Period(period)
	if err != nil {
		return Key{}, err
	}
	return d.derive(kind, ssp, string(kind), source, control, component, p)
}

// Attestation identifies a claim about one control, on one component, for one
// period. The attesting party is deliberately not in the key: fold it in and
// two peers attesting the same control period derive two identifiers, and
// federated deduplication fails completely. Consumers deduplicate on
// (object UUID, originating party) instead — see threat model TM-6.
func (d Deriver) Attestation(parentSSP string, src Source, controlID, componentUUID, period string) (Key, error) {
	return d.controlScoped(KindAttestation, parentSSP, src, controlID, componentUUID, period)
}

// Observation identifies an assessment observation. Its native `expires` field
// drives every countdown in the HUD and is not part of the key.
func (d Deriver) Observation(parentSSP string, src Source, controlID, componentUUID, period string) (Key, error) {
	return d.controlScoped(KindObservation, parentSSP, src, controlID, componentUUID, period)
}

// Finding identifies an assessment finding.
func (d Deriver) Finding(parentSSP string, src Source, controlID, componentUUID, period string) (Key, error) {
	return d.controlScoped(KindFinding, parentSSP, src, controlID, componentUUID, period)
}

// Risk identifies a risk. Whether it blocks an ATO is a prop on the object,
// not a key field: a risk that stops blocking is the same risk.
func (d Deriver) Risk(parentSSP string, src Source, controlID, componentUUID, period string) (Key, error) {
	return d.controlScoped(KindRisk, parentSSP, src, controlID, componentUUID, period)
}

// POAMItem identifies a plan-of-action-and-milestones item.
func (d Deriver) POAMItem(parentSSP string, src Source, controlID, componentUUID, period string) (Key, error) {
	return d.controlScoped(KindPOAMItem, parentSSP, src, controlID, componentUUID, period)
}

// EvidenceResource identifies a back-matter evidence resource by the hash of
// its content, not by a period: the same bytes submitted twice are the same
// resource, which is what a back-matter hash already claims. It takes no
// source-uuid, because bytes are not scoped to a catalog.
//
// The hash is the lowercase hex SHA-256 of the content.
func (d Deriver) EvidenceResource(parentSSP, sha256Hex string) (Key, error) {
	ssp, err := canonical.UUID(parentSSP)
	if err != nil {
		return Key{}, fmt.Errorf("parent ssp: %w", err)
	}
	h, err := canonicalSHA256(sha256Hex)
	if err != nil {
		return Key{}, err
	}
	return d.derive(KindResource, ssp, string(KindResource), h)
}

// AODecision identifies an authorizing official's decision. It is keyed on the
// risk it decides and the **day** it was taken, and takes no source-uuid for
// the same reason the risk's own key already carries one.
//
// A day rather than a period: a decision happens on a date, and the value has
// to line up with the `next-decision-date` prop the HUD counts down to.
func (d Deriver) AODecision(parentSSP, riskUUID, decisionDate string) (Key, error) {
	ssp, err := canonical.UUID(parentSSP)
	if err != nil {
		return Key{}, fmt.Errorf("parent ssp: %w", err)
	}
	risk, err := canonical.UUID(riskUUID)
	if err != nil {
		return Key{}, fmt.Errorf("risk: %w", err)
	}
	day, err := canonical.DecisionDate(decisionDate)
	if err != nil {
		return Key{}, err
	}
	return d.derive(KindDecision, ssp, string(KindDecision), risk, day)
}

// Responsibility identifies one half of an inherited control: the provider's
// declared responsibility or the consumer's satisfaction of it. The two halves
// are different objects and must not collide, which is what the trailing half
// token guarantees.
func (d Deriver) Responsibility(parentSSP string, src Source, controlID, componentUUID string, half Half) (Key, error) {
	if half != HalfProvider && half != HalfConsumer {
		return Key{}, fmt.Errorf("keys: responsibility half %q is neither provider nor consumer", half)
	}
	ssp, err := canonical.UUID(parentSSP)
	if err != nil {
		return Key{}, fmt.Errorf("parent ssp: %w", err)
	}
	source, err := src.canonical()
	if err != nil {
		return Key{}, err
	}
	control, err := canonical.ControlID(src.Vocabulary, controlID)
	if err != nil {
		return Key{}, err
	}
	component, err := canonical.UUID(componentUUID)
	if err != nil {
		return Key{}, fmt.Errorf("component: %w", err)
	}
	return d.derive(KindResponsibility, ssp, string(KindResponsibility), source, control, component, string(half))
}

// CellForControl identifies a materialised projection cell whose column is a
// single control.
//
// Projection cells are materialised rather than exchanged, so their
// identifiers are local. They use the same grammar anyway, because an
// identifier scheme with an exception is an identifier scheme someone will use
// inconsistently.
func (d Deriver) CellForControl(nodeUUID string, src Source, controlID string, bucket Bucket) (Key, error) {
	control, err := canonical.ControlID(src.Vocabulary, controlID)
	if err != nil {
		return Key{}, err
	}
	return d.cell(nodeUUID, src, control, bucket)
}

// CellForFamily identifies a materialised projection cell whose column is a
// control family. It is the same row of the field-list table as
// CellForControl — the document's fourth field is "control-id | family-id" —
// split into two entry points so the caller states which vocabulary rule it
// means rather than the package guessing from the shape of the string.
func (d Deriver) CellForFamily(nodeUUID string, src Source, familyID string, bucket Bucket) (Key, error) {
	family, err := canonical.FamilyID(src.Vocabulary, familyID)
	if err != nil {
		return Key{}, err
	}
	return d.cell(nodeUUID, src, family, bucket)
}

func (d Deriver) cell(nodeUUID string, src Source, axis string, bucket Bucket) (Key, error) {
	node, err := canonical.UUID(nodeUUID)
	if err != nil {
		return Key{}, fmt.Errorf("node: %w", err)
	}
	source, err := src.canonical()
	if err != nil {
		return Key{}, err
	}
	b, err := bucket.canonical()
	if err != nil {
		return Key{}, err
	}
	return d.derive(KindCell, node, string(KindCell), source, axis, b)
}
