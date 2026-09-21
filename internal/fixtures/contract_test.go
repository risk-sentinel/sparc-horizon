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

// requirement is what an observation's assessed-control link resolves to: the
// control identifier, and the document that says which catalog defines it.
type requirement struct {
	control string
	source  string
}

// controlVocabularies reads the class off every control in both catalogs, as a
// consumer holding only the exported documents would have to. This is the
// resolution `source-uuid` exists to make possible.
func controlVocabularies(t *testing.T, tree Tree) map[string]canonical.Vocabulary {
	t.Helper()

	vocab := map[string]canonical.Vocabulary{}
	for _, p := range []string{pathNISTCatalog, pathSecurityHubCatalog} {
		cat := decode[oscal.OscalCompleteSchema](t, tree, p).Catalog
		for _, group := range *cat.Groups {
			for _, c := range *group.Controls {
				vocab[c.ID] = canonical.VocabOpaque
				if c.Class == "SP800-53" {
					vocab[c.ID] = canonical.VocabNIST80053
				}
			}
		}
	}
	return vocab
}

// inheritedRequirements are the ones reachable through the inherited
// component's own component definition, with the source that document states
// on its control-implementation — rule 1 in docs/03-data-model.md.
func inheritedRequirements(t *testing.T, tree Tree) map[string]requirement {
	t.Helper()

	out := map[string]requirement{}
	cdef := decode[oscal.OscalCompleteSchema](t, tree, pathComponentDefinition).ComponentDefinition
	for _, component := range *cdef.Components {
		for _, ci := range *component.ControlImplementations {
			source := strings.TrimPrefix(ci.Source, "#")
			for _, ir := range ci.ImplementedRequirements {
				out[ir.UUID] = requirement{control: ir.ControlId, source: source}
			}
		}
	}
	return out
}

// localRequirements are the boundary's own, which carry no `source` and so
// fall back to the SSP's import-profile — rule 2.
func localRequirements(t *testing.T, ssp *oscal.SystemSecurityPlan) map[string]requirement {
	t.Helper()

	profileSource := strings.TrimPrefix(ssp.ImportProfile.Href, "#")
	found := false
	for _, r := range *ssp.BackMatter.Resources {
		if r.UUID == profileSource {
			found = true
		}
	}
	if !found {
		t.Errorf("%s: import-profile names %s, which is not in back-matter", ssp.UUID, profileSource)
	}

	out := map[string]requirement{}
	for _, ir := range ssp.ControlImplementation.ImplementedRequirements {
		out[ir.UUID] = requirement{control: ir.ControlId, source: profileSource}
	}
	return out
}

// The audit claim in docs/09-acceptance.md, at the identifier level: every
// keyed object must recompute from the exported documents alone.
//
// Nothing here reads the generator's state. The observation names the
// implemented requirement it assesses; that requirement lives either in the
// boundary's SSP or in the inherited component's own component definition; and
// the vocabulary comes from the class on the control in the catalog the source
// resolves to.
func TestObservationIdentifiersRecomputeFromTheDocuments(t *testing.T) {
	tree := generate(t)
	d := keys.New(keys.ProvisionalNamespace())

	vocab := controlVocabularies(t, tree)
	if want := len(NISTControls) + len(SecurityHubControls); len(vocab) != want {
		t.Fatalf("resolved %d control classes, want %d", len(vocab), want)
	}
	inherited := inheritedRequirements(t, tree)

	recomputed := 0
	for _, b := range Boundaries {
		ssp := decode[oscal.OscalCompleteSchema](t, tree, pathSSP(b)).SystemSecurityPlan
		local := localRequirements(t, ssp)

		ar := decode[oscal.OscalCompleteSchema](t, tree, pathAssessmentResults(b)).AssessmentResults
		for _, obs := range *ar.Results[0].Observations {
			if recomputeObservation(t, d, obs, ssp.UUID, local, inherited, vocab) {
				recomputed++
			}
		}
	}
	if want := len(Boundaries) * (len(NISTControls) + len(SecurityHubControls)); recomputed != want {
		t.Errorf("recomputed %d observations, want %d", recomputed, want)
	}
}

// recomputeObservation re-derives one observation's UUID from the documents and
// reports whether it matched.
func recomputeObservation(
	t *testing.T,
	d keys.Deriver,
	obs oscal.Observation,
	sspUUID string,
	local, inherited map[string]requirement,
	vocab map[string]canonical.Vocabulary,
) bool {
	t.Helper()

	href := strings.TrimPrefix((*obs.Links)[0].Href, "#")
	req, ok := local[href]
	if !ok {
		req, ok = inherited[href]
	}
	if !ok {
		t.Errorf("observation %s names requirement %s, which no document defines", obs.UUID, href)
		return false
	}
	v, ok := vocab[req.control]
	if !ok {
		t.Errorf("requirement names control %q, which no catalog defines", req.control)
		return false
	}

	key, err := d.Observation(sspUUID, keys.Source{UUID: req.source, Vocabulary: v}, req.control,
		(*obs.Subjects)[0].SubjectUuid, Period)
	if err != nil {
		t.Errorf("%s: %v", req.control, err)
		return false
	}
	if key.UUID.String() != obs.UUID {
		t.Errorf("%s: recomputed %s, document says %s", req.control, key.UUID, obs.UUID)
		return false
	}
	return true
}

// propCensus is what a walk of the fixtures found, split by the only
// distinction that matters to the namespace schema: props it validates, props
// from an authority Horizon does not own, and props with no `ns` at all.
type propCensus struct {
	sparc    []oscal.Property
	foreign  int
	absentNs int
}

func censusProps(t *testing.T, tree Tree) propCensus {
	t.Helper()

	var census propCensus
	for _, p := range tree.Paths() {
		if !strings.HasPrefix(p, "oscal/") {
			continue
		}
		var doc any
		if err := json.Unmarshal(tree[p], &doc); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		walkProps(doc, func(prop map[string]any) {
			ns, has := prop["ns"].(string)
			switch {
			case !has:
				census.absentNs++
			case ns != NamespaceSPARC:
				census.foreign++
			default:
				name, _ := prop["name"].(string)
				value, _ := prop["value"].(string)
				census.sparc = append(census.sparc, oscal.Property{Name: name, Ns: ns, Value: value})
			}
		})
	}
	return census
}

// walkProps visits every entry of every props array in a decoded document.
func walkProps(v any, visit func(map[string]any)) {
	switch x := v.(type) {
	case map[string]any:
		if list, ok := x["props"].([]any); ok {
			for _, p := range list {
				if prop, ok := p.(map[string]any); ok {
					visit(prop)
				}
			}
		}
		for _, val := range x {
			walkProps(val, visit)
		}
	case []any:
		for _, e := range x {
			walkProps(e, visit)
		}
	}
}

var (
	propUUIDRe    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[45][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	propDateRe    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	propTriggerRe = regexp.MustCompile(`^(score|blockers)(<|<=|>|>=)[0-9.]+$`)

	propEnums = map[string][]string{
		PropNodeType:     {"federation", "organization", "boundary", "system"},
		PropFIPS199:      {"low", "moderate", "high"},
		PropBlocksATO:    {"true", "false"},
		PropEvidenceKind: {"manual-attestation", "hdf-results", "scan-report", "document", "screenshot"},
	}
	propPatterns = map[string]*regexp.Regexp{
		PropParentUUID:       propUUIDRe,
		PropSignedBy:         propUUIDRe,
		PropNextDecisionDate: propDateRe,
		PropConditionExpires: propDateRe,
		PropTrigger:          propTriggerRe,
	}
)

// Every prop in the namespace Horizon owns, checked against the contract the
// schema states. The ajv job in contracts.yml validates the same props against
// the schema itself; this is the half that can fail a Go test, and it also
// asserts that all nine names are exercised.
func TestSPARCNamespacePropsHonourTheContract(t *testing.T) {
	census := censusProps(t, generate(t))

	seen := map[string]int{}
	for _, prop := range census.sparc {
		seen[prop.Name]++
		if allowed, ok := propEnums[prop.Name]; ok && !contains(allowed, prop.Value) {
			t.Errorf("prop %s has value %q, outside its enumeration", prop.Name, prop.Value)
		}
		if re, ok := propPatterns[prop.Name]; ok && !re.MatchString(prop.Value) {
			t.Errorf("prop %s has value %q, which does not match its pattern", prop.Name, prop.Value)
		}
	}

	for _, name := range []string{
		PropNodeType, PropParentUUID, PropNextDecisionDate, PropFIPS199, PropBlocksATO,
		PropEvidenceKind, PropSignedBy, PropConditionExpires, PropTrigger,
	} {
		if seen[name] == 0 {
			t.Errorf("prop %q never appears in the fixtures; the contract is not being exercised", name)
		}
	}

	// The two cases the schema must be seen not to fire on: a prop from an
	// authority Horizon does not own, and a prop with no `ns` at all, which
	// OSCAL reads as the default NIST namespace.
	if census.foreign == 0 {
		t.Error("no foreign-namespace prop in the fixtures")
	}
	if census.absentNs == 0 {
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

type boundaryRow struct {
	ID           int    `json:"id"`
	Slug         string `json:"slug"`
	OSCALSSPUUID string `json:"oscal_ssp_uuid"`
}

// Both addressing schemes on one object, which is what lets P1's client be
// exercised honestly: SPARC addresses a boundary by numeric id and slug, and
// every row carries the OSCAL UUID the documents join on.
func TestSPARCRowsCarryTheOSCALUUIDs(t *testing.T) {
	tree := generate(t)
	g := New(keys.ProvisionalNamespace())

	var rows struct {
		Count int           `json:"count"`
		Data  []boundaryRow `json:"data"`
	}
	if err := json.Unmarshal(tree[pathSPARC("authorization_boundaries")], &rows); err != nil {
		t.Fatalf("authorization_boundaries: %v", err)
	}
	if rows.Count != len(Boundaries) || len(rows.Data) != len(Boundaries) {
		t.Fatalf("%d rows (count says %d), want %d", len(rows.Data), rows.Count, len(Boundaries))
	}

	for i, row := range rows.Data {
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
		if got, want := metadataProp(t, ssp, PropParentUUID), g.orgPartyUUID(b.Org()); got != want {
			t.Errorf("%s: parent-uuid is %s, want the organization party %s", b.Slug, got, want)
		}
	}
}

func metadataProp(t *testing.T, ssp *oscal.SystemSecurityPlan, name string) string {
	t.Helper()

	if ssp.Metadata.Props == nil {
		t.Fatalf("%s: no metadata props", ssp.UUID)
	}
	for _, p := range *ssp.Metadata.Props {
		if p.Name == name {
			return p.Value
		}
	}
	return ""
}

// providerResponsibilities maps control id to the responsibility UUID the
// provider declared for it.
func providerResponsibilities(ssp *oscal.SystemSecurityPlan) map[string]string {
	out := map[string]string{}
	for _, ir := range ssp.ControlImplementation.ImplementedRequirements {
		for _, bc := range *ir.ByComponents {
			if bc.Export == nil || bc.Export.Responsibilities == nil {
				continue
			}
			out[ir.ControlId] = (*bc.Export.Responsibilities)[0].UUID
		}
	}
	return out
}

// consumerAnswers maps control id to the responsibility UUID this boundary
// answers for it.
func consumerAnswers(ssp *oscal.SystemSecurityPlan) map[string]string {
	out := map[string]string{}
	for _, ir := range ssp.ControlImplementation.ImplementedRequirements {
		for _, bc := range *ir.ByComponents {
			if bc.Satisfied == nil {
				continue
			}
			out[ir.ControlId] = (*bc.Satisfied)[0].ResponsibilityUuid
		}
	}
	return out
}

// The inherited control is the one case where two boundaries have to agree on
// an identifier neither of them derived alone.
func TestInheritedControlHalvesJoinAcrossBoundaries(t *testing.T) {
	tree := generate(t)
	provider := decode[oscal.OscalCompleteSchema](t, tree, pathSSP(providerBoundary())).SystemSecurityPlan

	declared := providerResponsibilities(provider)
	if len(declared) != len(InheritedControls) {
		t.Fatalf("the provider exports %d responsibilities, want %d", len(declared), len(InheritedControls))
	}

	answered := 0
	for _, b := range Boundaries {
		if b.Slug == ProviderBoundarySlug {
			continue
		}
		ssp := decode[oscal.OscalCompleteSchema](t, tree, pathSSP(b)).SystemSecurityPlan
		for control, got := range consumerAnswers(ssp) {
			want, inherited := declared[control]
			if !inherited {
				t.Errorf("%s/%s: answers a responsibility the provider never declared", b.Slug, control)
				continue
			}
			if got != want {
				t.Errorf("%s/%s: answers responsibility %s, the provider declared %s", b.Slug, control, got, want)
			}
			answered++
		}
	}
	if want := (len(Boundaries) - 1) * len(InheritedControls); answered != want {
		t.Errorf("%d consumer halves, want %d", answered, want)
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
