package view

import (
	"regexp"
	"sort"
	"strings"
	"time"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-3"

	"github.com/risk-sentinel/sparc-horizon/internal/project"
)

// assessment is one control's state on one component, recovered from the
// documents.
type assessment struct {
	boundary    *boundary
	requirement string
	controlID   string
	family      string
	component   string
	expires     *time.Time
	failing     bool
	blocksATO   bool
	// nist marks a control the boundary's own SSP implements, which puts it
	// on the 800-53 axis. A control that reaches the boundary only through a
	// component definition is another authority's identifier (AWS Security
	// Hub in the fixtures) and stays off the axis until SPARC's crosswalk
	// maps it (sparc#1103).
	nist        bool
	inherited   bool
	resource    string
	observation string
	finding     string
	risk        string
	poamItem    string
}

func (a assessment) state(at time.Time) project.State {
	return project.StateAt(project.Control{
		Failing:   a.failing,
		Expires:   a.expires,
		BlocksATO: a.blocksATO,
		Inherited: a.inherited,
	}, at)
}

// objectiveSuffix strips an objective or statement id down to its control:
// "ac-6.1_obj" → "ac-6.1", "ac-2_smt.a" → "ac-2", "EC2.2_obj" → "EC2.2".
var objectiveSuffix = regexp.MustCompile(`^(.+?)_(obj|smt)(\..*)?$`)

func controlOfTarget(t oscal.FindingTarget) string {
	if m := objectiveSuffix.FindStringSubmatch(t.TargetId); m != nil {
		return m[1]
	}
	return t.TargetId
}

// normal is the form two control ids are compared in: "AC-2 (1)", "ac-2.1"
// and "AC-2.1" are one control.
func normal(id string) string {
	id = strings.ToLower(strings.ReplaceAll(id, " ", ""))
	id = strings.ReplaceAll(id, "(", ".")
	return strings.ReplaceAll(id, ")", "")
}

func familyOf(controlID string) string {
	if i := strings.IndexAny(controlID, "-."); i > 0 {
		return strings.ToUpper(controlID[:i])
	}
	return strings.ToUpper(controlID)
}

// assessor joins one boundary's documents.
type assessor struct {
	b        *boundary
	set      *Set
	cdefReqs map[string]string // normal control id -> component-definition requirement uuid
	byKey    map[string]*assessment
	order    []string
}

func (v *View) assess(b *boundary) []assessment {
	as := &assessor{b: b, set: v.set, cdefReqs: v.cdefReqs, byKey: map[string]*assessment{}}
	for _, d := range b.ars {
		for _, r := range d.Doc.AssessmentResults.Results {
			as.result(d, r)
		}
	}
	as.poams()
	as.sspFacts()

	out := make([]assessment, 0, len(as.order))
	for _, k := range as.order {
		out = append(out, *as.byKey[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].controlID != out[j].controlID {
			return out[i].controlID < out[j].controlID
		}
		return out[i].component < out[j].component
	})
	return out
}

// result reads one assessment result's findings. A later result overrides an
// earlier one for the same control and component; within one result a
// not-satisfied finding outranks a satisfied one.
func (as *assessor) result(d *Document, r oscal.Result) {
	obs := map[string]oscal.Observation{}
	for _, o := range deref(r.Observations) {
		obs[o.UUID] = o
	}
	risks := map[string]oscal.Risk{}
	for _, rk := range deref(r.Risks) {
		risks[rk.UUID] = rk
	}

	seen := map[string]bool{}
	for _, f := range deref(r.Findings) {
		control := controlOfTarget(f.Target)
		if control == "" {
			as.set.note(d.Path, "finding %s has no target-id; skipped", f.UUID)
			continue
		}
		related := as.observations(d, f, obs)
		for _, component := range as.components(related) {
			a := as.fromFinding(f, control, component, related, risks)
			k := normal(control) + "\x00" + component
			if prev, ok := as.byKey[k]; ok && seen[k] && prev.failing && !a.failing {
				continue
			}
			if _, ok := as.byKey[k]; !ok {
				as.order = append(as.order, k)
			}
			seen[k] = true
			as.byKey[k] = &a
		}
	}
}

func (as *assessor) observations(d *Document, f oscal.Finding, obs map[string]oscal.Observation) []oscal.Observation {
	var out []oscal.Observation
	for _, ro := range deref(f.RelatedObservations) {
		o, ok := obs[ro.ObservationUuid]
		if !ok {
			as.set.note(d.Path, "finding %s names observation %s, which its result does not carry", f.UUID, ro.ObservationUuid)
			continue
		}
		out = append(out, o)
	}
	return out
}

// components are the systems a finding is about, from its observations'
// subjects. An inventory item stands for the component it implements. A
// finding naming no component is kept against the boundary as a whole.
func (as *assessor) components(related []oscal.Observation) []string {
	var out []string
	seen := map[string]bool{}
	addOnce := func(id string) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, o := range related {
		for _, s := range deref(o.Subjects) {
			switch s.Type {
			case "component":
				addOnce(s.SubjectUuid)
			case "inventory-item":
				for _, c := range as.implemented(s.SubjectUuid) {
					addOnce(c)
				}
			}
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func (as *assessor) implemented(item string) []string {
	var out []string
	for _, it := range deref(as.b.plan.SystemImplementation.InventoryItems) {
		if it.UUID != item {
			continue
		}
		for _, ic := range deref(it.ImplementedComponents) {
			out = append(out, ic.ComponentUuid)
		}
	}
	return out
}

func (as *assessor) fromFinding(f oscal.Finding, control, component string, related []oscal.Observation, risks map[string]oscal.Risk) assessment {
	a := assessment{
		boundary:  as.b,
		controlID: control,
		family:    familyOf(control),
		component: component,
		failing:   f.Target.Status.State == "not-satisfied",
	}
	for _, o := range related {
		if a.observation == "" {
			a.observation = o.UUID
		}
		if a.resource == "" {
			for _, ev := range deref(o.RelevantEvidence) {
				a.resource = strings.TrimPrefix(ev.Href, "#")
				break
			}
		}
		// The earliest expiry governs: the claim is only as current as its
		// stalest observation.
		if o.Expires != nil && (a.expires == nil || o.Expires.Before(*a.expires)) {
			e := *o.Expires
			a.expires = &e
		}
	}
	if a.failing {
		a.finding = f.UUID
		for _, rr := range deref(f.RelatedRisks) {
			a.risk = rr.RiskUuid
			a.blocksATO = blocksATO(risks[rr.RiskUuid])
			break
		}
	}
	return a
}

// blocksATO reads SPARC's blocks-ato prop, which turns a down control into a
// red cell rather than an amber one.
func blocksATO(r oscal.Risk) bool {
	for _, p := range deref(r.Props) {
		if p.Name == "blocks-ato" && p.Value == "true" {
			return true
		}
	}
	return false
}

// poams attaches POA&M items by the finding, risk or observation they name.
func (as *assessor) poams() {
	for _, d := range as.b.poams {
		for _, item := range d.Doc.PlanOfActionAndMilestones.PoamItems {
			as.attach(item.UUID, itemRefs(item))
		}
	}
}

// itemRefs is every finding, risk and observation a POA&M item names.
func itemRefs(item oscal.PoamItem) map[string]bool {
	refs := map[string]bool{}
	for _, rf := range deref(item.RelatedFindings) {
		refs[rf.FindingUuid] = true
	}
	for _, rr := range deref(item.RelatedRisks) {
		refs[rr.RiskUuid] = true
	}
	for _, ro := range deref(item.RelatedObservations) {
		refs[ro.ObservationUuid] = true
	}
	return refs
}

func (as *assessor) attach(item string, refs map[string]bool) {
	for _, k := range as.order {
		a := as.byKey[k]
		if a.poamItem == "" && a.failing && (refs[a.finding] || refs[a.risk] || refs[a.observation]) {
			a.poamItem = item
		}
	}
}

// sspFacts records what only the SSP knows: the implemented requirement a
// control is (the chain's key), that it is the boundary's own, and whether it
// is implemented through inheritance. A control the SSP does not implement
// keys its chain on a component definition's requirement, else on its
// finding.
func (as *assessor) sspFacts() {
	irs := map[string]oscal.ImplementedRequirement{}
	for _, ir := range as.b.plan.ControlImplementation.ImplementedRequirements {
		irs[normal(ir.ControlId)] = ir
	}
	for _, k := range as.order {
		a := as.byKey[k]
		if ir, ok := irs[normal(a.controlID)]; ok {
			a.requirement = ir.UUID
			a.controlID = ir.ControlId
			a.family = familyOf(ir.ControlId)
			a.nist = true
			for _, bc := range deref(ir.ByComponents) {
				if bc.Inherited != nil {
					a.inherited = true
				}
			}
			continue
		}
		if req, ok := as.cdefReqs[normal(a.controlID)]; ok {
			a.requirement = req
			continue
		}
		a.requirement = a.finding
		if a.requirement == "" {
			a.requirement = a.observation
		}
	}
}
