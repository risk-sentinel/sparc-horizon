package oscal

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/defenseunicorns/go-oscal/src/pkg/validation"
	oscal122 "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"
)

// The gap the probe found, pinned from both directions.
//
// OSCAL defines `local-definitions` four times with four different shapes, and
// gives three of them the same schema title, "Local Definitions". `go-oscal`
// names generated types from that title, so the three collapse into one struct
// carrying the assessment-plan shape. The fourth is a named `$ref`, which is
// why `PlanOfActionAndMilestonesLocalDefinitions` exists and the others do not.
//
// | Where | OSCAL allows |
// |---|---|
// | assessment-plan | activities, components, inventory-items, objectives-and-methods, remarks, users |
// | assessment-results (document) | activities, objectives-and-methods, remarks |
// | result (inside results[]) | assessment-assets, components, inventory-items, tasks, users |
// | plan-of-action-and-milestones | assessment-assets, components, inventory-items, remarks |
//
// Identical in the OSCAL 1.2.3 schema, and in `go-oscal`'s unreleased 1.2.3
// types, so this is not something a version bump resolves.

const authoredResultLocalDefinitions = "testdata/authored/result-local-definitions.json"

// Direction one: a schema-valid document loses fields on the way in.
func TestResultLocalDefinitionsLosesTasksAndAssessmentAssets(t *testing.T) {
	b, err := os.ReadFile(authoredResultLocalDefinitions)
	if err != nil {
		t.Fatalf("reading the authored document: %v", err)
	}

	// The premise: OSCAL accepts this document. Without checking that, a
	// missing field could just as easily mean the document was wrong.
	v, err := validation.NewValidator(b)
	if err != nil {
		t.Fatalf("building a validator: %v", err)
	}
	if err := v.Validate(); err != nil {
		t.Fatalf("the authored document is not valid OSCAL, so it proves nothing: %v", err)
	}

	ts, err := TypesFor("1.2.2")
	if err != nil {
		t.Fatalf("TypesFor: %v", err)
	}
	diffs, err := RoundTrip(b, ts)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}

	lost := []string{}
	for _, d := range Only(diffs, DiffOther) {
		lost = append(lost, d.Path)
	}
	sort.Strings(lost)

	want := []string{
		"/assessment-results/results/0/local-definitions/assessment-assets",
		"/assessment-results/results/0/local-definitions/tasks",
	}
	if strings.Join(lost, ",") != strings.Join(want, ",") {
		// Both outcomes are worth a person's attention: a third lost field is
		// a wider gap than recorded, and none at all means go-oscal fixed it
		// and the documentation here is now wrong.
		t.Errorf("lost %v, recorded %v — either the gap widened or it was fixed upstream; docs/10-risks-decisions.md says what this is", lost, want)
	}

	if err := StrictDecode(b, ts); err == nil {
		t.Error("DisallowUnknownFields accepted a document with fields the types do not model")
	}
}

// Direction two, which is the one a probe over other people's documents cannot
// find: the shared struct lets Horizon BUILD something OSCAL forbids.
//
// `components` is legal on an assessment plan's local-definitions and not on an
// assessment-results document's. One Go type for both means the compiler has
// nothing to say about it, and the first thing that objects is a schema
// validator — if anyone runs one.
func TestDocumentLevelLocalDefinitionsAcceptsWhatOSCALForbids(t *testing.T) {
	doc := oscal122.OscalCompleteSchema{AssessmentResults: &oscal122.AssessmentResults{
		UUID:     "9f2a1c3d-5e7b-4a91-8c62-0d4e8b1a7f35",
		ImportAp: oscal122.ImportAp{Href: "#assessment-plan"},
		Metadata: oscal122.Metadata{
			Title:        "Document-level local-definitions",
			Version:      "1.0.0",
			OscalVersion: "1.2.2",
			LastModified: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
		},
		LocalDefinitions: &oscal122.LocalDefinitions{
			// Legal on an assessment plan. Not legal here.
			Components: &[]oscal122.SystemComponent{{
				UUID:        "3a5c9e21-8b74-4f06-9d13-27e8a4c05b6f",
				Type:        "software",
				Title:       "A component",
				Description: "Legal on an assessment plan's local-definitions, not on this one.",
				Status:      oscal122.SystemComponentStatus{State: "operational"},
			}},
		},
		Results: []oscal122.Result{{
			UUID:        "2c8f4b16-77d3-4a08-9e51-3b6ac0192d84",
			Title:       "r",
			Description: "d",
			Start:       time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			ReviewedControls: oscal122.ReviewedControls{ControlSelections: []oscal122.AssessedControls{{
				IncludeControls: &[]oscal122.AssessedControlsSelectControlById{{ControlId: "ac-2"}},
			}}},
		}},
	}}

	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("the typed structs would not even marshal it: %v", err)
	}

	v, err := validation.NewValidator(b)
	if err != nil {
		t.Fatalf("building a validator: %v", err)
	}
	err = v.Validate()
	if err == nil {
		t.Fatal("OSCAL accepted components on an assessment-results document's local-definitions; the premise of this test is wrong")
	}
	if !strings.Contains(err.Error(), "/assessment-results/local-definitions") {
		t.Errorf("the schema objected, but not where expected: %v", err)
	}

	// Stated as the standing instruction rather than left implicit: until the
	// types distinguish the four assemblies, a schema validation is the only
	// thing between Horizon and an invalid emitted document.
	t.Log("go-oscal's types produced a document its own validator rejects — validate anything Horizon emits")
}
