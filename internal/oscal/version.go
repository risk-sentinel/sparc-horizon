// Package oscal holds Horizon's OSCAL document handling.
//
// What is here today is the round-trip fidelity probe from #26, re-run against
// documents this repository did not author (#49). The adapters P1 builds on
// top of it are not here yet.
//
// The probe is in the tree rather than in a scratch directory because the
// question it answers does not stay answered: `go-oscal` is pinned, but a
// version bump, a new OSCAL revision, or a document from a peer at a version
// nobody anticipated all change the result. Horizon's audit test is that every
// cell recomputes from exported OSCAL alone, so a type layer that silently
// drops a field breaks that test the first time someone needs the field.
package oscal

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/go-oscal/src/pkg/model"
	"github.com/defenseunicorns/go-oscal/src/pkg/versioning"
	oscal112 "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-2"
	oscal113 "github.com/defenseunicorns/go-oscal/src/types/oscal-1-1-3"
	oscal121 "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-1"
	oscal122 "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"
)

// TypeSet is the typed struct family for one OSCAL version.
//
// `go-oscal` ships a separate package per version, so selecting one is a
// decision rather than an import. Published content makes that unavoidable:
// NIST's own examples declare 1.1.2, 1.1.3 and 1.2.2 across the models Horizon
// reads, while Horizon's fixtures are 1.2.2.
type TypeSet struct {
	Version string
	// newDocument returns a pointer to an empty complete-schema document of
	// this version, ready to unmarshal into.
	newDocument func() any
}

// SupportedVersions are the versions Horizon decodes. The list is short on
// purpose: it is the set actually observed in content this prototype is built
// against, and an unlisted version is rejected rather than decoded with the
// nearest types. Guessing is how a field goes missing without anyone noticing.
var SupportedVersions = []string{"1.1.2", "1.1.3", "1.2.1", "1.2.2"}

// TypesFor returns the type set for an OSCAL version.
func TypesFor(version string) (TypeSet, error) {
	switch version {
	case "1.1.2":
		return TypeSet{Version: version, newDocument: func() any { return &oscal112.OscalCompleteSchema{} }}, nil
	case "1.1.3":
		return TypeSet{Version: version, newDocument: func() any { return &oscal113.OscalCompleteSchema{} }}, nil
	case "1.2.1":
		return TypeSet{Version: version, newDocument: func() any { return &oscal121.OscalCompleteSchema{} }}, nil
	case "1.2.2":
		return TypeSet{Version: version, newDocument: func() any { return &oscal122.OscalCompleteSchema{} }}, nil
	default:
		return TypeSet{}, fmt.Errorf("oscal: version %q is not one of %v", version, SupportedVersions)
	}
}

// Version reads the OSCAL version a document declares, through `go-oscal`'s own
// helpers rather than a second implementation of the same lookup.
func Version(b []byte) (string, error) {
	m, err := model.CoerceToJsonMap(b)
	if err != nil {
		return "", fmt.Errorf("oscal: %w", err)
	}
	v, err := versioning.GetOscalVersionFromMap(m)
	if err != nil {
		return "", fmt.Errorf("oscal: %w", err)
	}
	return versioning.FormatOscalVersion(v), nil
}

// KnownModels are the OSCAL models this estate exchanges. Horizon reads the
// first seven; assessment-plan is here because published content carries it and
// a probe that cannot name a document cannot report on it.
var KnownModels = []string{
	"catalog",
	"profile",
	"component-definition",
	"system-security-plan",
	"assessment-plan",
	"assessment-results",
	"plan-of-action-and-milestones",
	"mapping-collection",
}

// Model reads which OSCAL model a document is — "system-security-plan",
// "catalog", and so on.
//
// `go-oscal`'s own helper returns whatever single top-level key it finds, so
// {"nonsense": {}} is a valid answer to it. That is the right behaviour for a
// generic tool and the wrong one here: a document whose root is not an OSCAL
// model is not a document Horizon can do anything with, and naming it anyway
// pushes the failure downstream to whoever trusted the name.
func Model(b []byte) (string, error) {
	m, err := model.CoerceToJsonMap(b)
	if err != nil {
		return "", fmt.Errorf("oscal: %w", err)
	}
	t, err := model.GetModelType(m)
	if err != nil {
		return "", fmt.Errorf("oscal: %w", err)
	}
	for _, known := range KnownModels {
		if t == known {
			return t, nil
		}
	}
	return "", fmt.Errorf("oscal: %q is not an OSCAL model root", t)
}

// StrictDecode decodes a document with unknown fields refused, and reports what
// the types do not model.
//
// This is the sharper of the two checks. A round trip compares what survived;
// this says whether anything in the document was never represented at all.
func StrictDecode(b []byte, ts TypeSet) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(ts.newDocument()); err != nil {
		return fmt.Errorf("oscal: strict decode at %s: %w", ts.Version, err)
	}
	return nil
}
