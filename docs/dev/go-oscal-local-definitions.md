# `go-oscal` collapses OSCAL's four `local-definitions` assemblies

**Status: written, not filed.** This is a defect in a third-party library
([`defenseunicorns/go-oscal`](https://github.com/defenseunicorns/go-oscal)), not in this estate.
Filing it upstream is the repository owner's call, so it is recorded here with its evidence
rather than sitting in someone's notes. Everything below the line is written to be filed as-is.

Found by the round-trip fidelity probe in [`internal/oscal`](../../internal/oscal) while
discharging the re-test obligation from #26 (see #49). The consequences for Horizon, and the two
standing instructions that follow from them, are in
[`docs/10-risks-decisions.md`](../10-risks-decisions.md); this document is the upstream report.

---

## Summary

`go-oscal` generates one `LocalDefinitions` struct for three structurally different OSCAL
assemblies, because it derives type identity from the schema's `title` and OSCAL gives all three
the title `"Local Definitions"`.

The result is silent data loss in one direction and invalid output in the other:

- A **schema-valid** assessment-results document loses `results[*].local-definitions.tasks` and
  `results[*].local-definitions.assessment-assets` on a parse-and-re-serialise cycle. The
  re-serialised document **still validates**, so schema validation before and after both pass
  while content has gone.
- The same struct accepts `components`, `inventory-items` and `users` on an *assessment-results
  document*, where OSCAL forbids them — so the typed structs will build and marshal a document
  that `go-oscal`'s own validator rejects.

## Affected

| | |
|---|---|
| Released | **v0.7.1**, all type packages `oscal-1-0-4` through `oscal-1-2-2` |
| Unreleased | `main`, including the `oscal-1-2-3` types |
| Not a version-skew issue | The OSCAL 1.2.2 and 1.2.3 schemas are identical in this area |

## What OSCAL specifies

`local-definitions` appears four times, with four different property sets. Three are inline
(anonymous) assemblies; the fourth is a named definition.

| Parent | Properties allowed | In the schema |
|---|---|---|
| `assessment-plan` | activities, components, inventory-items, objectives-and-methods, remarks, users | inline |
| `assessment-results` (document level) | activities, objectives-and-methods, remarks | inline |
| `result` (inside `results[]`) | **assessment-assets**, components, inventory-items, **tasks**, users | inline |
| `plan-of-action-and-milestones` | assessment-assets, components, inventory-items, remarks | `$ref` to `oscal-complete-oscal-poam:local-definitions` |

All three inline assemblies carry `"title": "Local Definitions"`.

## What `go-oscal` generates

One struct, carrying the **assessment-plan** shape, used for all three inline positions:

```go
type LocalDefinitions struct {
    Activities           *[]Activity        `json:"activities,omitempty"`
    Components           *[]SystemComponent `json:"components,omitempty"`
    InventoryItems       *[]InventoryItem   `json:"inventory-items,omitempty"`
    ObjectivesAndMethods *[]LocalObjective  `json:"objectives-and-methods,omitempty"`
    Remarks              string             `json:"remarks,omitempty"`
    Users                *[]SystemUser      `json:"users,omitempty"`
}
```

The fourth is generated correctly, as `PlanOfActionAndMilestonesLocalDefinitions` — which is
also the evidence that the intended naming convention exists and simply is not reached for the
other three.

## Why

`getRef` in `src/internal/generate/generate_utils.go` establishes a schema's identity:

```go
func getRef(schema jsonschema.Schema) (string, error) {
    if schema.Ref != nil {
        return *schema.Ref, nil
    } else if schema.ID != nil {
        return *schema.ID, nil
    } else if schema.Title != nil {
        return getRefWithName(getNameFromTitle(*schema.Title)), nil   // <- here
    }
    return "", fmt.Errorf("unable to get ref from schema")
}
```

An anonymous assembly has no `$ref` and no `$id`, so its identity becomes
`#/definitions/` + its title. All three inline `local-definitions` therefore resolve to the
**same synthetic ref**, `#/definitions/LocalDefinitions`.

`handleDuplicates` in `src/internal/generate/generate.go` then never engages, because its first
test is whether the ref differs:

```go
if currentRef, ok := c.nameMap[name]; ok {
    // Points to a different definition
    if currentRef != ref {
        ...
```

The refs are equal, so the three are treated as one definition and the first one generated wins.
The POA&M assembly escapes precisely because it has a real `$ref`, which differs, so it takes the
disambiguation path and is prefixed with its parent's name.

**This is a naming-collision problem only in appearance. It is an identity problem:** for an
anonymous assembly, identity is derived from a field OSCAL does not guarantee to be unique.

## Scope

Three of the 41 inline assembly property names in the OSCAL 1.2.2 schema carry more than one
shape:

| Property | Outcome |
|---|---|
| `entries` | **Correct.** OSCAL titles them `Assessment Log Entry` and `Risk Log Entry`, so they get distinct types |
| `status` | **Correct.** Distinct titles again — `ObjectiveStatus`, `SystemComponentStatus` |
| `local-definitions` | **Collapsed.** One title, three shapes |

So this is currently one instance. It is worth fixing at the generator, because nothing stops
OSCAL reusing a title again in a future revision, and the failure mode is silent.

## Impact, measured

Run against `usnistgov/oscal-content`'s own published example
(`examples/ar/json/ifa_assessment-results.json`, which declares OSCAL 1.2.2):

```
round trip:    /assessment-results/results/0/local-definitions/tasks  -> absent
strict decode: json: unknown field "tasks"
```

And against a minimal authored document using both fields OSCAL allows there:

```
1. the document is valid OSCAL 1.2.2, by go-oscal's own validator   -> valid
2. round trip                                                       -> local-definitions: {}
   the re-serialised document                                       -> still VALID OSCAL
3. DisallowUnknownFields                                            -> unknown field "assessment-assets"
4. typed construction of a document-level local-definitions
   with `components`                                                -> schema rejects:
   at '/assessment-results/local-definitions': additional properties 'components' not allowed
```

Step 2 is the one that matters most. The library validates the input, loses two assemblies, and
produces output that validates again — so a pipeline that schema-validates at both ends sees
green twice and has silently dropped the assessment activities and assets that a result recorded.

## Suggested fixes

In increasing order of what actually solves it:

1. **Add the missing fields to `LocalDefinitions`.** Fixes the data loss and makes the second
   direction worse: one union type would then accept even more than OSCAL permits in any single
   position. Not sufficient alone.
2. **Generate the three types separately** — `AssessmentPlanLocalDefinitions`,
   `AssessmentResultsLocalDefinitions`, `ResultLocalDefinitions` — and point each parent at its
   own. The convention already exists in `PlanOfActionAndMilestonesLocalDefinitions`. This is a
   breaking change to the generated API and worth a major note.
3. **Fix the identity rule** so an anonymous assembly is identified by its *location* in the
   schema rather than by its title — for example the parent definition name plus the property
   name — falling back to the title only when that is unambiguous. This removes the class of
   defect rather than this instance, and would produce the same names as (2).
4. **Add a regression test** costing nothing to source: decode
   `usnistgov/oscal-content`'s `examples/ar/json/ifa_assessment-results.json` with
   `DisallowUnknownFields` and assert it succeeds. It fails today.

## Reproducer

Self-contained; no dependency beyond `go-oscal` itself.

```bash
mkdir repro && cd repro && go mod init repro
go get github.com/defenseunicorns/go-oscal@v0.7.1
# save the program below as main.go
go run .
```

```go
// Standalone reproducer for the local-definitions type collapse in go-oscal.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/defenseunicorns/go-oscal/src/pkg/validation"
	oscalTypes "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"
)

// A minimal, schema-valid assessment-results document whose result-level
// local-definitions uses the two fields OSCAL allows there.
const doc = `{
  "assessment-results": {
    "uuid": "9f2a1c3d-5e7b-4a91-8c62-0d4e8b1a7f35",
    "metadata": {"title": "repro", "last-modified": "2026-09-21T00:00:00Z", "version": "1.0.0", "oscal-version": "1.2.2"},
    "import-ap": {"href": "#assessment-plan"},
    "results": [{
      "uuid": "2c8f4b16-77d3-4a08-9e51-3b6ac0192d84",
      "title": "r", "description": "d", "start": "2026-09-01T00:00:00Z",
      "reviewed-controls": {"control-selections": [{"include-controls": [{"control-id": "ac-2"}]}]},
      "local-definitions": {
        "assessment-assets": {"assessment-platforms": [{"uuid": "6d1e8a92-4c05-4f7e-b3a8-91c2d7460fe1", "title": "Scanning platform"}]},
        "tasks": [{"uuid": "b470c5e8-2f19-4d63-8a07-5e9314cb2d60", "type": "action", "title": "t", "description": "d"}]
      }
    }]
  }
}`

func main() {
	fmt.Println("== 1. the document is valid OSCAL 1.2.2, by go-oscal's own validator ==")
	v, err := validation.NewValidator([]byte(doc))
	if err != nil {
		panic(err)
	}
	if err := v.Validate(); err != nil {
		fmt.Println("   unexpectedly invalid:", err)
		return
	}
	fmt.Println("   valid")

	fmt.Println("\n== 2. round trip loses two fields, and the result still validates ==")
	var parsed oscalTypes.OscalCompleteSchema
	if err := json.Unmarshal([]byte(doc), &parsed); err != nil {
		panic(err)
	}
	after, err := json.Marshal(parsed)
	if err != nil {
		panic(err)
	}
	var got map[string]any
	if err := json.Unmarshal(after, &got); err != nil {
		panic(err)
	}
	results := got["assessment-results"].(map[string]any)["results"].([]any)
	ld, present := results[0].(map[string]any)["local-definitions"]
	fmt.Printf("   local-definitions after round trip: present=%v value=%v\n", present, ld)

	rv, err := validation.NewValidator(after)
	if err != nil {
		panic(err)
	}
	if err := rv.Validate(); err != nil {
		fmt.Println("   the re-serialised document is INVALID:", err)
	} else {
		fmt.Println("   ...and the re-serialised document is still VALID OSCAL.")
		fmt.Println("   So the loss is silent: schema validation before and after both pass.")
	}

	fmt.Println("\n== 3. DisallowUnknownFields names the fields the types do not model ==")
	dec := json.NewDecoder(bytes.NewReader([]byte(doc)))
	dec.DisallowUnknownFields()
	var strict oscalTypes.OscalCompleteSchema
	fmt.Println("  ", dec.Decode(&strict))

	fmt.Println("\n== 4. the same struct emits a document OSCAL forbids ==")
	invalid := oscalTypes.OscalCompleteSchema{AssessmentResults: &oscalTypes.AssessmentResults{
		UUID:     "9f2a1c3d-5e7b-4a91-8c62-0d4e8b1a7f35",
		ImportAp: oscalTypes.ImportAp{Href: "#assessment-plan"},
		Metadata: oscalTypes.Metadata{Title: "repro", Version: "1.0.0", OscalVersion: "1.2.2",
			LastModified: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)},
		// `components` is legal on an assessment plan's local-definitions and
		// not on an assessment-results document's. One Go type for both means
		// nothing objects until a schema validator does.
		LocalDefinitions: &oscalTypes.LocalDefinitions{Components: &[]oscalTypes.SystemComponent{{
			UUID: "3a5c9e21-8b74-4f06-9d13-27e8a4c05b6f", Type: "software", Title: "c",
			Description: "d", Status: oscalTypes.SystemComponentStatus{State: "operational"},
		}}},
		Results: []oscalTypes.Result{{
			UUID: "2c8f4b16-77d3-4a08-9e51-3b6ac0192d84", Title: "r", Description: "d",
			Start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			ReviewedControls: oscalTypes.ReviewedControls{ControlSelections: []oscalTypes.AssessedControls{{
				IncludeControls: &[]oscalTypes.AssessedControlsSelectControlById{{ControlId: "ac-2"}},
			}}},
		}},
	}}
	emitted, err := json.Marshal(invalid)
	if err != nil {
		panic(err)
	}
	ev, err := validation.NewValidator(emitted)
	if err != nil {
		panic(err)
	}
	err = ev.Validate()
	if err == nil {
		fmt.Println("   schema accepted it")
		return
	}
	for _, line := range bytes.Split([]byte(err.Error()), []byte("\n")) {
		if bytes.Contains(line, []byte("local-definitions")) {
			fmt.Printf("  %s\n", bytes.TrimSpace(line))
		}
	}
}
```

To confirm the four shapes independently, against the schema `go-oscal` vendors:

```bash
python3 - <<'PY'
import json, glob
schema = glob.glob('**/oscal_complete_schema-1-2-2.json', recursive=True)[0]
defs = json.load(open(schema))['definitions']
for k, v in sorted(defs.items()):
    ld = (v.get('properties') or {}).get('local-definitions')
    if isinstance(ld, dict) and 'properties' in ld:
        print(f"{k.split(':')[-1]:22} title={ld.get('title')!r:20} {sorted(ld['properties'])}")
PY
```

## Environment

| | |
|---|---|
| `go-oscal` | v0.7.1 (module proxy), and `main` as of 2026-09-21 |
| Go | 1.25.13 and 1.26.3, same result |
| OSCAL | 1.2.2 and 1.2.3 schemas, same shapes |
| Corpus | `usnistgov/oscal-content` at `78650f02ad9321bb7b817846f8fbd4f2bcd620de` |
