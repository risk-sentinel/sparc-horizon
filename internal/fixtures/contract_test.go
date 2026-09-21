package fixtures

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"

	"github.com/risk-sentinel/sparc-horizon/internal/canonical"
	"github.com/risk-sentinel/sparc-horizon/internal/keys"
)

func decode[T any](t *testing.T, tree Tree, path string) T {
	t.Helper()
	var doc T
	if err := json.Unmarshal(tree[path], &doc); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return doc
}

// The audit claim in docs/09-acceptance.md, at the identifier level: every
// keyed object must recompute from the exported documents alone.
//
// Nothing here reads the generator's state. The observation names the
// implemented requirement it assesses; that requirement lives either in the
// boundary's SSP — where `source-uuid` falls back to import-profile — or in
// the inherited component's own component definition, which carries `source`
// directly. The vocabulary comes from the class on the control in the catalog
// the source resolves to, which is what the qualifier exists to make
// decidable.
func TestObservationIdentifiersRecomputeFromTheDocuments(t *testing.T) {
	tree := generate(t)
	d := keys.New(keys.ProvisionalNamespace())

	vocab := map[string]canonical.Vocabulary{}
	for _, p := range []string{pathNISTCatalog, pathSecurityHubCatalog} {
		cat := decode[oscal.OscalCompleteSchema](t, tree, p).Catalog
		for _, group := range *cat.Groups {
			for _, c := range *group.Controls {
				if c.Class == "SP800-53" {
					vocab[c.ID] = canonical.VocabNIST80053
					continue
				}
				vocab[c.ID] = canonical.VocabOpaque
			}
		}
	}
	if len(vocab) != len(NISTControls)+len(SecurityHubControls) {
		t.Fatalf("resolved %d control classes, want %d", len(vocab), len(NISTControls)+len(SecurityHubControls))
	}

	// Requirements reachable through the inherited component definition, with
	// the source that document states on its control-implementation. This is
	// rule 1 in docs/03-data-model.md; the SSP path below is rule 2.
	type requirement struct {
		control string
		source  string
	}
	requirements := map[string]requirement{}
	cdef := decode[oscal.OscalCompleteSchema](t, tree, pathComponentDefinition).ComponentDefinition
	for _, component := range *cdef.Components {
		for _, ci := range *component.ControlImplementations {
			source := strings.TrimPrefix(ci.Source, "#")
			for _, ir := range ci.ImplementedRequirements {
				requirements[ir.UUID] = requirement{control: ir.ControlId, source: source}
			}
		}
	}

	recomputed := 0
	for _, b := range Boundaries {
		ssp := decode[oscal.OscalCompleteSchema](t, tree, pathSSP(b)).SystemSecurityPlan
		profileSource := strings.TrimPrefix(ssp.ImportProfile.Href, "#")

		// The href must name a back-matter resource that is really there.
		found := false
		for _, r := range *ssp.BackMatter.Resources {
			if r.UUID == profileSource {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: import-profile names %s, which is not in back-matter", b.Slug, profileSource)
		}

		local := map[string]requirement{}
		for _, ir := range ssp.ControlImplementation.ImplementedRequirements {
			local[ir.UUID] = requirement{control: ir.ControlId, source: profileSource}
		}

		ar := decode[oscal.OscalCompleteSchema](t, tree, pathAssessmentResults(b)).AssessmentResults
		for _, obs := range *ar.Results[0].Observations {
			href := strings.TrimPrefix((*obs.Links)[0].Href, "#")
			req, ok := local[href]
			if !ok {
				req, ok = requirements[href]
			}
			if !ok {
				t.Errorf("%s: observation %s names requirement %s, which no document defines", b.Slug, obs.UUID, href)
				continue
			}
			v, ok := vocab[req.control]
			if !ok {
				t.Errorf("%s: requirement names control %q, which no catalog defines", b.Slug, req.control)
				continue
			}
			component := (*obs.Subjects)[0].SubjectUuid

			key, err := d.Observation(ssp.UUID, keys.Source{UUID: req.source, Vocabulary: v}, req.control, component, Period)
			if err != nil {
				t.Errorf("%s/%s: %v", b.Slug, req.control, err)
				continue
			}
			if key.UUID.String() != obs.UUID {
				t.Errorf("%s/%s: recomputed %s, document says %s", b.Slug, req.control, key.UUID, obs.UUID)
			}
			recomputed++
		}
	}
	if want := len(Boundaries) * (len(NISTControls) + len(SecurityHubControls)); recomputed != want {
		t.Errorf("recomputed %d observations, want %d", recomputed, want)
	}
}

// Every prop in the namespace Horizon owns, checked against the contract the
// schema states. The ajv job in contracts.yml validates the same props against
// the schema itself; this is the half that can fail a Go test, and it also
// asserts that all nine names are actually exercised.
func TestSPARCNamespacePropsHonourTheContract(t *testing.T) {
	tree := generate(t)

	var (
		uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[45][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
		dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
		trigRe = regexp.MustCompile(`^(score|blockers)(<|<=|>|>=)[0-9.]+$`)
		enums  = map[string][]string{
			"node-type":     {"federation", "organization", "boundary", "system"},
			"fips-199":      {"low", "moderate", "high"},
			"blocks-ato":    {"true", "false"},
			"evidence-kind": {"manual-attestation", "hdf-results", "scan-report", "document", "screenshot"},
		}
	)

	seen := map[string]int{}
	foreign, absentNs := 0, 0
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				if k == "props" {
					list, _ := val.([]any)
					for _, p := range list {
						prop, _ := p.(map[string]any)
						ns, has := prop["ns"].(string)
						switch {
						case !has:
							absentNs++
							continue
						case ns != NamespaceSPARC:
							foreign++
							continue
						}
						name, _ := prop["name"].(string)
						value, _ := prop["value"].(string)
						seen[name]++

						if allowed, ok := enums[name]; ok {
							if !contains(allowed, value) {
								t.Errorf("prop %s has value %q, outside its enumeration", name, value)
							}
						}
						switch name {
						case "parent-uuid", "signed-by":
							if !uuidRe.MatchString(value) {
								t.Errorf("prop %s has value %q, which is not a UUID", name, value)
							}
						case "next-decision-date", "condition-expires":
							if !dateRe.MatchString(value) {
								t.Errorf("prop %s has value %q, which is not a date", name, value)
							}
						case "trigger":
							if !trigRe.MatchString(value) {
								t.Errorf("prop %s has value %q, which is not a trigger expression", name, value)
							}
						}
					}
				}
				walk(val)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}

	for _, p := range tree.Paths() {
		if !strings.HasPrefix(p, "oscal/") {
			continue
		}
		var doc any
		if err := json.Unmarshal(tree[p], &doc); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		walk(doc)
	}

	for _, name := range []string{
		"node-type", "parent-uuid", "next-decision-date", "fips-199",
		"blocks-ato", "evidence-kind", "signed-by", "condition-expires", "trigger",
	} {
		if seen[name] == 0 {
			t.Errorf("prop %q never appears in the fixtures; the contract is not being exercised", name)
		}
	}

	// The two cases the schema must be seen not to fire on: a prop from an
	// authority Horizon does not own, and a prop with no `ns` at all, which
	// OSCAL reads as the default NIST namespace.
	if foreign == 0 {
		t.Error("no foreign-namespace prop in the fixtures")
	}
	if absentNs == 0 {
		t.Error("no prop with an absent ns in the fixtures")
	}
}

// Pass-through preservation is a correctness requirement, not a courtesy: a
// foreign prop is re-emitted with its name, namespace and value exactly as
// issued, CamelCase included.
func TestForeignPropsArePreservedAsIssued(t *testing.T) {
	tree := generate(t)
	ssp := decode[oscal.OscalCompleteSchema](t, tree, pathSSP(Boundaries[0])).SystemSecurityPlan

	var inherited *oscal.SystemComponent
	for i, c := range ssp.SystemImplementation.Components {
		if c.Title == InheritedComponentName {
			inherited = &ssp.SystemImplementation.Components[i]
		}
	}
	if inherited == nil {
		t.Fatal("the inherited platform component is not in the SSP")
	}

	want := map[string]string{"EvaluatedServices": NamespaceAWS, "SeverityLabel": NamespaceAWS, "implementation-point": ""}
	got := map[string]string{}
	for _, p := range *inherited.Props {
		got[p.Name] = p.Ns
	}
	for name, ns := range want {
		if have, ok := got[name]; !ok || have != ns {
			t.Errorf("prop %q: ns %q, want %q (present: %v)", name, have, ns, ok)
		}
	}
}

// Both addressing schemes on one object, which is what lets P1's client be
// exercised honestly: SPARC addresses a boundary by numeric id and slug, and
// every row carries the OSCAL UUID the documents join on.
func TestSPARCRowsCarryTheOSCALUUIDs(t *testing.T) {
	tree := generate(t)
	g := New(keys.ProvisionalNamespace())

	var boundaries struct {
		Count int `json:"count"`
		Data  []struct {
			ID           int    `json:"id"`
			Slug         string `json:"slug"`
			OSCALSSPUUID string `json:"oscal_ssp_uuid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(tree[pathSPARC("authorization_boundaries")], &boundaries); err != nil {
		t.Fatalf("authorization_boundaries: %v", err)
	}
	if boundaries.Count != len(Boundaries) || len(boundaries.Data) != len(Boundaries) {
		t.Fatalf("%d rows (count says %d), want %d", len(boundaries.Data), boundaries.Count, len(Boundaries))
	}
	for i, row := range boundaries.Data {
		b := Boundaries[i]
		if row.Slug != b.Slug || row.ID != b.ID {
			t.Errorf("row %d addresses %d/%s, want %d/%s", i, row.ID, row.Slug, b.ID, b.Slug)
		}
		ssp := decode[oscal.OscalCompleteSchema](t, tree, pathSSP(b)).SystemSecurityPlan
		if row.OSCALSSPUUID != ssp.UUID {
			t.Errorf("%s: row points at %s, the SSP is %s", b.Slug, row.OSCALSSPUUID, ssp.UUID)
		}
		if ssp.SystemCharacteristics.SystemIds[0].ID != b.Slug {
			t.Errorf("%s: the SSP does not carry SPARC's slug", b.Slug)
		}
		if ssp.Metadata.Props == nil {
			t.Fatalf("%s: no metadata props", b.Slug)
		}
		parent := ""
		for _, p := range *ssp.Metadata.Props {
			if p.Name == "parent-uuid" {
				parent = p.Value
			}
		}
		if want := g.orgPartyUUID(b.Org()); parent != want {
			t.Errorf("%s: parent-uuid is %s, want the organization party %s", b.Slug, parent, want)
		}
	}
}

// The inherited control is the one case where two boundaries have to agree on
// an identifier neither of them derived alone.
func TestInheritedControlHalvesJoinAcrossBoundaries(t *testing.T) {
	tree := generate(t)
	provider := decode[oscal.OscalCompleteSchema](t, tree, pathSSP(providerBoundary())).SystemSecurityPlan

	responsibilities := map[string]string{} // control id -> responsibility uuid
	for _, ir := range provider.ControlImplementation.ImplementedRequirements {
		for _, bc := range *ir.ByComponents {
			if bc.Export == nil || bc.Export.Responsibilities == nil {
				continue
			}
			responsibilities[ir.ControlId] = (*bc.Export.Responsibilities)[0].UUID
		}
	}
	if len(responsibilities) != len(InheritedControls) {
		t.Fatalf("the provider exports %d responsibilities, want %d", len(responsibilities), len(InheritedControls))
	}

	consumers := 0
	for _, b := range Boundaries {
		if b.Slug == ProviderBoundarySlug {
			continue
		}
		ssp := decode[oscal.OscalCompleteSchema](t, tree, pathSSP(b)).SystemSecurityPlan
		for _, ir := range ssp.ControlImplementation.ImplementedRequirements {
			want, inherited := responsibilities[ir.ControlId]
			if !inherited {
				continue
			}
			for _, bc := range *ir.ByComponents {
				if bc.Satisfied == nil {
					continue
				}
				if got := (*bc.Satisfied)[0].ResponsibilityUuid; got != want {
					t.Errorf("%s/%s: answers responsibility %s, the provider declared %s", b.Slug, ir.ControlId, got, want)
				}
				consumers++
			}
		}
	}
	if want := (len(Boundaries) - 1) * len(InheritedControls); consumers != want {
		t.Errorf("%d consumer halves, want %d", consumers, want)
	}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
