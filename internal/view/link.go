package view

import (
	"path"
	"strings"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-3"

	"github.com/risk-sentinel/sparc-horizon/internal/project"
)

// boundary is one SSP and the documents that resolved to it.
type boundary struct {
	ssp   *Document
	plan  *oscal.SystemSecurityPlan
	slug  string
	name  string
	fips  string
	ars   []*Document
	poams []*Document
}

func (b *boundary) id() string { return b.plan.UUID }

// weight is the boundary's FIPS 199 rollup weight.
func (b *boundary) weight() float64 { return project.FIPSWeight(b.fips) }

// linker resolves references between documents.
type linker struct {
	set        *Set
	byUUID     map[string]*Document
	boundaries []*boundary
	bySSP      map[string]*boundary
}

func link(set *Set) *linker {
	l := &linker{set: set, byUUID: map[string]*Document{}, bySSP: map[string]*boundary{}}
	for _, d := range set.Docs {
		if id := d.UUID(); id != "" {
			l.byUUID[id] = d
		}
	}

	slugs := map[string]bool{}
	for _, d := range set.ofModel(modelSSP) {
		b := &boundary{ssp: d, plan: d.Doc.SystemSecurityPlan}
		b.name = b.plan.SystemCharacteristics.SystemName
		b.slug = strings.TrimPrefix(d.stem(), "ssp-")
		if slugs[b.slug] {
			set.note(d.Path, "slug %q is taken by another SSP; this one is named by its UUID", b.slug)
			b.slug = b.plan.UUID
		}
		slugs[b.slug] = true
		b.fips = fipsOf(b.plan)
		if b.weight() == 0 {
			set.note(d.Path, "security-sensitivity-level %q is not low, moderate or high; weighted as low",
				b.plan.SystemCharacteristics.SecuritySensitivityLevel)
			b.fips = "low"
		}
		l.boundaries = append(l.boundaries, b)
		l.bySSP[b.id()] = b
	}

	for _, d := range set.ofModel(modelAR) {
		if b := l.resultsBoundary(d); b != nil {
			b.ars = append(b.ars, d)
		}
	}
	for _, d := range set.ofModel(modelPOAM) {
		if b := l.poamBoundary(d); b != nil {
			b.poams = append(b.poams, d)
		}
	}
	return l
}

// fipsOf reads the impact level: SPARC's fips-199 prop when present, else the
// SSP's security-sensitivity-level, in either "moderate" or "fips-199-moderate"
// form.
func fipsOf(ssp *oscal.SystemSecurityPlan) string {
	for _, p := range deref(ssp.Metadata.Props) {
		if p.Name == "fips-199" {
			return strings.ToLower(p.Value)
		}
	}
	level := strings.ToLower(ssp.SystemCharacteristics.SecuritySensitivityLevel)
	return strings.TrimPrefix(level, "fips-199-")
}

// resolve finds the document an href names: "#<uuid>", or a path whose file
// name (without extension, since a published set mixes XML and JSON
// references) matches exactly one loaded document of the model.
func (l *linker) resolve(href, model string) *Document {
	if id, ok := strings.CutPrefix(href, "#"); ok {
		if d := l.byUUID[id]; d != nil && d.Model == model {
			return d
		}
		return nil
	}
	base := path.Base(href)
	stem := strings.TrimSuffix(base, path.Ext(base))
	var found *Document
	for _, d := range l.set.ofModel(model) {
		if d.stem() == stem {
			if found != nil {
				return nil
			}
			found = d
		}
	}
	return found
}

// resultsBoundary places an assessment-results document: through its
// assessment plan, else by the components its observations name, else as the
// only SSP in the set. Every fallback is reported.
func (l *linker) resultsBoundary(d *Document) *boundary {
	ar := d.Doc.AssessmentResults
	if ap := l.resolve(ar.ImportAp.Href, modelAP); ap != nil {
		if ssp := l.resolve(ap.Doc.AssessmentPlan.ImportSsp.Href, modelSSP); ssp != nil {
			return l.bySSP[ssp.UUID()]
		}
		l.set.note(d.Path, "its assessment plan %s imports %q, which is not a loaded SSP", ap.Path, ap.Doc.AssessmentPlan.ImportSsp.Href)
	} else {
		l.set.note(d.Path, "import-ap %q is not a loaded assessment plan", ar.ImportAp.Href)
	}

	var subjects []string
	for _, r := range ar.Results {
		for _, o := range deref(r.Observations) {
			for _, s := range deref(o.Subjects) {
				subjects = append(subjects, s.SubjectUuid)
			}
		}
	}
	return l.fallback(d, subjects)
}

// poamBoundary places a POA&M: import-ssp, then system-id, then the fallbacks.
func (l *linker) poamBoundary(d *Document) *boundary {
	p := d.Doc.PlanOfActionAndMilestones
	if p.ImportSsp != nil {
		if ssp := l.resolve(p.ImportSsp.Href, modelSSP); ssp != nil {
			return l.bySSP[ssp.UUID()]
		}
		l.set.note(d.Path, "import-ssp %q is not a loaded SSP", p.ImportSsp.Href)
	}
	if p.SystemId != nil {
		if b := l.bySystemID(p.SystemId.ID); b != nil {
			l.set.note(d.Path, "placed in %s by system-id", b.ssp.Path)
			return b
		}
	}
	var subjects []string
	for _, o := range deref(p.Observations) {
		for _, s := range deref(o.Subjects) {
			subjects = append(subjects, s.SubjectUuid)
		}
	}
	return l.fallback(d, subjects)
}

func (l *linker) bySystemID(id string) *boundary {
	for _, b := range l.boundaries {
		for _, sid := range b.plan.SystemCharacteristics.SystemIds {
			if sid.ID == id {
				return b
			}
		}
	}
	return nil
}

// fallback places a document by the SSP whose components it names most, or,
// naming none, by the only SSP loaded. Ambiguity is refused and reported.
func (l *linker) fallback(d *Document, subjects []string) *boundary {
	best, bestCount, tie := (*boundary)(nil), 0, false
	for _, b := range l.boundaries {
		n := 0
		for _, s := range subjects {
			if hasComponent(b.plan, s) {
				n++
			}
		}
		switch {
		case n > bestCount:
			best, bestCount, tie = b, n, false
		case n == bestCount && n > 0:
			tie = true
		}
	}
	if best != nil && !tie {
		l.set.note(d.Path, "placed in %s by the components its observations name", best.ssp.Path)
		return best
	}
	if len(l.boundaries) == 1 {
		l.set.note(d.Path, "placed in %s, the only SSP loaded", l.boundaries[0].ssp.Path)
		return l.boundaries[0]
	}
	l.set.note(d.Path, "could not be placed in any SSP; not shown")
	return nil
}

func hasComponent(ssp *oscal.SystemSecurityPlan, id string) bool {
	for _, c := range ssp.SystemImplementation.Components {
		if c.UUID == id {
			return true
		}
	}
	for _, item := range deref(ssp.SystemImplementation.InventoryItems) {
		if item.UUID == id {
			return true
		}
	}
	return false
}

func deref[T any](p *[]T) []T {
	if p == nil {
		return nil
	}
	return *p
}
