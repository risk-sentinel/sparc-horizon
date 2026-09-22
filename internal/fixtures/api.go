package fixtures

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"

	"github.com/risk-sentinel/sparc-horizon/internal/project"
)

// The API v0 mock's golden responses, built from the OSCAL this same run
// emitted — parsed back out of the documents rather than read from the
// generator's memory.
//
// That is deliberate and it is the cheap version of the audit claim: every
// number the mock serves recomputes from exported OSCAL alone. If a response
// cannot be derived that way, it does not belong in the contract either.
//
// # These are illustrative shape, not projections
//
// Cell state comes from project.StateAt over the fixtures' own observation
// expiries and blocking risks. That is the documented algorithm, but it is not
// the projection engine: there is no ledger, no materialisation, no
// invalidation and no ranking model. P2 builds those. Nothing here may be
// cited as an engine result.

// Horizon is the date the golden responses project to. Fixed, like everything
// else in this tree: a mock whose answers move with the clock is a mock nobody
// can write a test against.
var Horizon = days(30)

// Persona is a caller the mock can serve. Roles bind to a node and inherit
// downward, so each persona sees a different tree rather than the same tree
// with parts greyed out.
type Persona struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	PartyUUID string `json:"party-uuid"`
	NodeUUID  string `json:"node-uuid"`
	NodeType  string `json:"node-type"`
	Name      string `json:"name"`
	About     string `json:"about"`
}

// APINode is a node in the tree the caller can see.
type APINode struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	NodeType string    `json:"nodeType"`
	Roles    []string  `json:"roles"`
	Children []APINode `json:"children,omitempty"`
}

// APICell is one cell of a heat row.
type APICell struct {
	Col       string `json:"col"`
	State     string `json:"state"`
	Blockers  int    `json:"blockers"`
	Worsening bool   `json:"worsening"`
	Inherited bool   `json:"inherited"`
}

// APIRow is one child of the node being viewed.
type APIRow struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Cells []APICell `json:"cells"`
}

// APIHeat is the heatmap response.
type APIHeat struct {
	Node      string   `json:"node"`
	Horizon   string   `json:"horizon"`
	Axis      string   `json:"axis"`
	Columns   []string `json:"columns"`
	Collapsed []string `json:"collapsed"`
	Rows      []APIRow `json:"rows"`
}

// APIControl is one control in a cell drill-down, with why it is down.
type APIControl struct {
	ControlID string `json:"controlId"`
	Component string `json:"component"`
	State     string `json:"state"`
	Reason    string `json:"reason"`
	Expires   string `json:"expires,omitempty"`
	BlocksATO bool   `json:"blocksAto"`
	Chain     string `json:"chain"`
}

// APICellDetail is the cell drill-down response.
type APICellDetail struct {
	Node     string       `json:"node"`
	Col      string       `json:"col"`
	Horizon  string       `json:"horizon"`
	Controls []APIControl `json:"controls"`
}

// APIChain is the evidence chain for one implemented requirement, in the fixed
// order docs/03-data-model.md sets: resource, observation, finding, risk,
// POA&M item.
type APIChain struct {
	Requirement string `json:"requirement"`
	ControlID   string `json:"controlId"`
	Component   string `json:"component"`
	Resource    string `json:"resource,omitempty"`
	Observation string `json:"observation,omitempty"`
	Expires     string `json:"expires,omitempty"`
	Finding     string `json:"finding,omitempty"`
	Risk        string `json:"risk,omitempty"`
	BlocksATO   bool   `json:"blocksAto"`
	PoamItem    string `json:"poamItem,omitempty"`
}

// APINextAction is the single focal point a lens offers.
type APINextAction struct {
	Node        string `json:"node"`
	Title       string `json:"title"`
	Consequence string `json:"consequence"`
	ControlID   string `json:"controlId"`
	Chain       string `json:"chain"`
	Horizon     string `json:"horizon"`
}

// assessment is one control's projected state, recovered from the documents.
type assessment struct {
	boundary    Boundary
	requirement string
	controlID   string
	family      string
	component   string
	expires     time.Time
	failing     bool
	blocksATO   bool
	// nist marks a control resolved through the boundary's own profile.
	// Everything else arrives through an inherited component definition and
	// is keyed to another authority — AWS Security Hub here.
	nist        bool
	inherited   bool
	resource    string
	observation string
	finding     string
	risk        string
	poamItem    string
}

func (a assessment) state() project.State {
	return project.StateAt(project.Control{
		Failing:   a.failing,
		Expires:   &a.expires,
		BlocksATO: a.blocksATO,
		Inherited: a.inherited,
	}, Horizon)
}

// Personas are the callers the mock serves, matching P1's exit criteria: an AO
// at an organization, and an SO and an ISO at a boundary.
func (g *Generator) Personas() []Persona {
	org := Orgs[0]
	boundary, other := Boundaries[0], Boundaries[2]
	return []Persona{
		{
			ID: "ao-" + org.Slug, Role: roleAO,
			PartyUUID: g.rolePartyUUID(boundary, roleAO),
			NodeUUID:  g.orgPartyUUID(org), NodeType: "organization", Name: org.Name + " Authorizing Official",
			About: "Sees the organization, its boundaries and their systems. Roles inherit downward.",
		},
		{
			ID: "so-" + boundary.Slug, Role: roleSO,
			PartyUUID: g.rolePartyUUID(boundary, roleSO),
			NodeUUID:  g.sspUUID(boundary), NodeType: "boundary", Name: boundary.Name + " System Owner",
			About: "Sees one boundary and its systems. Everything above it is 404, not 403.",
		},
		{
			ID: "iso-" + other.Slug, Role: roleISO,
			PartyUUID: g.rolePartyUUID(other, roleISO),
			NodeUUID:  g.sspUUID(other), NodeType: "boundary", Name: other.Name + " Information System Security Officer",
			About: "A different organization entirely, to show that two personas do not overlap.",
		},
	}
}

// assessmentsFor recovers every control's projected state for a boundary, from
// the SSP and assessment-results this run emitted.
func (g *Generator) assessmentsFor(tree Tree, b Boundary) ([]assessment, error) {
	var ssp oscal.OscalCompleteSchema
	if err := json.Unmarshal(tree[pathSSP(b)], &ssp); err != nil {
		return nil, fmt.Errorf("%s ssp: %w", b.Slug, err)
	}
	var ar oscal.OscalCompleteSchema
	if err := json.Unmarshal(tree[pathAssessmentResults(b)], &ar); err != nil {
		return nil, fmt.Errorf("%s ar: %w", b.Slug, err)
	}

	// Which requirement each observation assessed, and the chain hanging off
	// it — the same link the recompute test in internal/oscal follows.
	result := ar.AssessmentResults.Results[0]
	byRequirement := map[string]*assessment{}
	for _, obs := range *result.Observations {
		req := strings.TrimPrefix((*obs.Links)[0].Href, "#")
		controlID := (*obs.Links)[0].ResourceFragment
		a := &assessment{
			boundary:    b,
			requirement: req,
			controlID:   controlID,
			family:      familyOf(controlID),
			component:   (*obs.Subjects)[0].SubjectUuid,
			observation: obs.UUID,
			resource:    strings.TrimPrefix((*obs.RelevantEvidence)[0].Href, "#"),
		}
		if obs.Expires != nil {
			a.expires = *obs.Expires
		}
		byRequirement[req] = a
	}

	// Findings and risks mark what is down and what blocks.
	obsToReq := map[string]string{}
	for req, a := range byRequirement {
		obsToReq[a.observation] = req
	}
	for _, f := range deref(result.Findings) {
		req, ok := obsToReq[(*f.RelatedObservations)[0].ObservationUuid]
		if !ok {
			continue
		}
		byRequirement[req].failing = true
		byRequirement[req].finding = f.UUID
		if f.RelatedRisks != nil {
			byRequirement[req].risk = (*f.RelatedRisks)[0].RiskUuid
		}
	}
	for _, r := range deref(result.Risks) {
		req, ok := obsToReq[(*r.RelatedObservations)[0].ObservationUuid]
		if !ok || r.Props == nil {
			continue
		}
		for _, p := range *r.Props {
			if p.Name == PropBlocksATO && p.Value == "true" {
				byRequirement[req].blocksATO = true
			}
		}
	}

	// POA&M items, and which requirements are inherited.
	var poam oscal.OscalCompleteSchema
	if err := json.Unmarshal(tree[pathPOAM(b)], &poam); err != nil {
		return nil, fmt.Errorf("%s poam: %w", b.Slug, err)
	}
	for _, item := range poam.PlanOfActionAndMilestones.PoamItems {
		if item.RelatedObservations == nil {
			continue
		}
		if req, ok := obsToReq[(*item.RelatedObservations)[0].ObservationUuid]; ok {
			byRequirement[req].poamItem = item.UUID
		}
	}
	for _, ir := range ssp.SystemSecurityPlan.ControlImplementation.ImplementedRequirements {
		a, ok := byRequirement[ir.UUID]
		if !ok {
			continue
		}
		// Present in this SSP's own control-implementation, so `source-uuid`
		// resolves through import-profile to the 800-53 baseline.
		a.nist = true
		for _, bc := range *ir.ByComponents {
			if bc.Inherited != nil {
				a.inherited = true
			}
		}
	}

	out := make([]assessment, 0, len(byRequirement))
	for _, a := range byRequirement {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].controlID != out[j].controlID {
			return out[i].controlID < out[j].controlID
		}
		return out[i].component < out[j].component
	})
	return out, nil
}

func familyOf(controlID string) string {
	if i := strings.IndexAny(controlID, "-."); i > 0 {
		return strings.ToUpper(controlID[:i])
	}
	return strings.ToUpper(controlID)
}

func deref[T any](p *[]T) []T {
	if p == nil {
		return nil
	}
	return *p
}
