package fixtures

import (
	"time"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-3"

	"github.com/risk-sentinel/sparc-horizon/internal/keys"
)

// The assessment side of a boundary: what was observed, what failed, what it
// costs, and when the evidence stops counting.
//
// Every choice here comes from the deterministic sequence, consumed in a fixed
// order, so the shape of the federation is stable while still varying enough
// to exercise the projection engine: observations that expire inside each
// horizon bucket, risks that block an ATO and risks that do not, and an
// accepted risk whose condition expires.

// assessed is one boundary's assessment plan, assessment-results and POA&M
// documents.
type assessed struct {
	plan    oscal.OscalCompleteSchema
	results oscal.OscalCompleteSchema
	poam    oscal.OscalCompleteSchema
}

// judgement is the generated verdict for one control on one component.
type judgement struct {
	control Control
	source  keys.Source
	// requirement is the UUID of the implemented-requirement this observation
	// assesses: in the boundary's SSP for a profile control, in the inherited
	// component's own component definition for a Security Hub one. It is what
	// ties an observation to a control identifier through the documents rather
	// than through the generator's memory.
	requirement string
	component   string
	satisfied   bool
	collected   time.Time
	expires     time.Time
	method      string
	evidence    evidenceArtifact
	blocksATO   bool
	accepted    bool
}

func (g *Generator) judgements(b Boundary, artifacts []evidenceArtifact) []judgement {
	scan, attestation := artifacts[0], artifacts[1]
	out := make([]judgement, 0, len(NISTControls)+len(SecurityHubControls))

	assess := func(c Control, source keys.Source, component, requirement string, manual bool) {
		j := judgement{
			control:     c,
			source:      source,
			requirement: requirement,
			component:   component,
			collected:   days(-g.rng.intn(21) - 1),
			method:      "TEST",
			evidence:    scan,
		}
		if manual {
			j.method = "EXAMINE"
			j.evidence = attestation
		}

		switch roll := g.rng.intn(100); {
		case roll < 68:
			// Current, and good for a while.
			j.satisfied = true
			j.expires = days(120 + g.rng.intn(280))
		case roll < 85:
			// Current, but decaying inside a horizon bucket. This is what the
			// HUD counts down to.
			j.satisfied = true
			j.expires = days(4 + g.rng.intn(24))
		default:
			j.satisfied = false
			j.expires = days(30 + g.rng.intn(60))
			// A failing control blocks an authorization more often where the
			// impact level is higher.
			threshold := map[string]int{"low": 15, "moderate": 35, "high": 60}[b.FIPS]
			j.blocksATO = g.rng.intn(100) < threshold
			j.accepted = !j.blocksATO && g.rng.intn(100) < 45
		}
		out = append(out, j)
	}

	for i, c := range NISTControls {
		assess(c, g.profileSource(),
			g.componentUUID(b, b.Systems[i%len(b.Systems)].Slug),
			g.objectUUID("implemented-requirement", b.Slug, c.ID),
			manualControls[c.ID])
	}
	for _, c := range SecurityHubControls {
		// The second authority reaches the boundary on the inherited platform
		// component, and its identifiers are not NIST's.
		assess(c, g.securityHubSource(),
			g.componentUUID(b, InheritedComponentSlug),
			g.objectUUID("cdef-requirement", InheritedComponentSlug, c.ID),
			false)
	}

	// Every boundary carries at least one failing control, so every
	// assessment-results and POA&M document in the tree exercises the whole
	// evidence chain. Left to the draw, a boundary can pass everything — which
	// is a fine thing for a real system to do and a poor fixture.
	failing := false
	for _, j := range out {
		if !j.satisfied {
			failing = true
			break
		}
	}
	if !failing {
		last := len(out) - 1
		out[last].satisfied = false
		out[last].expires = days(30 + g.rng.intn(60))
		out[last].blocksATO = b.FIPS == "high"
		out[last].accepted = false
	}
	return out
}

func (g *Generator) assess(b Boundary, artifacts []evidenceArtifact, sspBytes []byte) (assessed, error) {
	ssp := g.sspUUID(b)
	iso := g.rolePartyUUID(b, roleISO)
	ao := g.rolePartyUUID(b, roleAO)

	origin := []oscal.Origin{{Actors: []oscal.OriginActor{{
		Type:      "party",
		ActorUuid: iso,
		RoleId:    roleISO,
	}}}}

	resources, evidenceUUID, err := g.evidenceResources(ssp, iso, artifacts)
	if err != nil {
		return assessed{}, err
	}
	resources = append(resources,
		resourceFor(ssp, b.Name+" system security plan", "./ssp-"+b.Slug+".json", sha256Hex(sspBytes)),
	)

	var (
		observations []oscal.Observation
		findings     []oscal.Finding
		risks        []oscal.Risk
		poamRisks    []oscal.Risk
		attested     []oscal.AssessmentPart
		assessedIDs  []string
	)
	// poam-items is a required field, so it is an empty list rather than a
	// nil slice: a nil one marshals to null and makes the document invalid.
	poamItems := []oscal.PoamItem{}

	// The order below is load-bearing: the generator's random draws happen
	// inside shortfall, one judgement at a time, and the committed fixtures
	// depend on that sequence.
	for _, j := range g.judgements(b, artifacts) {
		assessedIDs = append(assessedIDs, j.control.ID)

		obs, obsUUID, err := g.observation(b, j, ssp, origin, evidenceUUID)
		if err != nil {
			return assessed{}, err
		}
		observations = append(observations, obs)

		if j.satisfied {
			finding, part, err := g.satisfied(b, j, ssp, origin, obsUUID)
			if err != nil {
				return assessed{}, err
			}
			findings = append(findings, finding)
			if part != nil {
				attested = append(attested, *part)
			}
			continue
		}

		sf, err := g.shortfall(b, j, ssp, iso, ao, origin, obsUUID)
		if err != nil {
			return assessed{}, err
		}
		findings = append(findings, sf.finding)
		risks = append(risks, sf.risk)
		poamRisks = append(poamRisks, sf.poamRisk)
		poamItems = append(poamItems, sf.item)
	}

	result := g.result(b, iso, assessedIDs, observations, findings, risks, attested)

	arResources := resources
	return assessed{
		plan: g.assessmentPlan(b, ssp, result.ReviewedControls),
		results: oscal.OscalCompleteSchema{AssessmentResults: &oscal.AssessmentResults{
			UUID:       g.documentUUID("assessment-results", b.Slug),
			Metadata:   g.assessmentMetadata(b, b.Name+" assessment results "+Period),
			ImportAp:   oscal.ImportAp{Href: "#" + g.documentUUID("assessment-plan", b.Slug)},
			Results:    []oscal.Result{result},
			BackMatter: &oscal.BackMatter{Resources: &arResources},
		}},
		poam: oscal.OscalCompleteSchema{PlanOfActionAndMilestones: &oscal.PlanOfActionAndMilestones{
			UUID:      g.documentUUID("poam", b.Slug),
			Metadata:  g.assessmentMetadata(b, b.Name+" plan of action and milestones "+Period),
			ImportSsp: &oscal.ImportSsp{Href: "#" + ssp},
			SystemId:  &oscal.SystemId{ID: b.Slug, IdentifierType: NamespaceSPARC + "/slug"},
			Risks:     opt(poamRisks),
			PoamItems: poamItems,
			BackMatter: &oscal.BackMatter{Resources: &[]oscal.Resource{
				resourceFor(ssp, b.Name+" system security plan", "./ssp-"+b.Slug+".json", sha256Hex(sspBytes)),
			}},
		}},
	}, nil
}

// result is the one assessment result for the period: the controls reviewed,
// the observations, findings and risks, and the ISO's attestations if any.
func (g *Generator) result(b Boundary, iso string, assessedIDs []string, observations []oscal.Observation,
	findings []oscal.Finding, risks []oscal.Risk, attested []oscal.AssessmentPart) oscal.Result {
	selections := make([]oscal.AssessedControlsSelectControlById, 0, len(assessedIDs))
	for _, id := range assessedIDs {
		selections = append(selections, oscal.AssessedControlsSelectControlById{ControlId: id})
	}
	result := oscal.Result{
		UUID:        g.objectUUID("result", b.Slug, Period),
		Title:       b.Name + " assessment " + Period,
		Description: "Continuous monitoring results for " + b.Name + ".",
		Start:       days(-21),
		End:         ptr(days(-1)),
		ReviewedControls: oscal.ReviewedControls{ControlSelections: []oscal.AssessedControls{{
			Description:     "Controls assessed for " + b.Name + " in " + Period + ".",
			IncludeControls: &selections,
		}}},
		Observations: opt(observations),
		Findings:     opt(findings),
		Risks:        opt(risks),
	}
	if len(attested) > 0 {
		result.Attestations = &[]oscal.AttestationStatements{{
			Parts: attested,
			ResponsibleParties: &[]oscal.ResponsibleParty{{
				RoleId:     roleISO,
				PartyUuids: []string{iso},
			}},
		}}
	}
	return result
}

// evidenceResources returns one back-matter resource per evidence artifact,
// keyed on the hash of its content: the same bytes submitted twice are the
// same resource. The map gives each artifact path's resource UUID.
func (g *Generator) evidenceResources(ssp, iso string, artifacts []evidenceArtifact) ([]oscal.Resource, map[string]string, error) {
	resources := make([]oscal.Resource, 0, len(artifacts)+2)
	evidenceUUID := map[string]string{}
	for _, a := range artifacts {
		key, err := g.keys.EvidenceResource(ssp, a.digest)
		if err != nil {
			return nil, nil, err
		}
		evidenceUUID[a.path] = key.UUID.String()
		resources = append(resources, oscal.Resource{
			UUID:        key.UUID.String(),
			Title:       a.title,
			Description: "Evidence artifact, hashed. Synthetic content; the digest is of the file as emitted.",
			Props: &[]oscal.Property{
				sparcProp(PropEvidenceKind, a.kind),
				sparcProp(PropSignedBy, iso),
			},
			Rlinks: &[]oscal.ResourceLink{{
				Href:      "../" + a.path,
				MediaType: "application/json",
				Hashes:    &[]oscal.Hash{{Algorithm: "SHA-256", Value: a.digest}},
			}},
		})
	}
	return resources, evidenceUUID, nil
}

// observation is one judgement's observation, and its UUID.
func (g *Generator) observation(b Boundary, j judgement, ssp string, origin []oscal.Origin, evidenceUUID map[string]string) (oscal.Observation, string, error) {
	obs, err := g.keys.Observation(ssp, j.source, j.control.ID, j.component, Period)
	if err != nil {
		return oscal.Observation{}, "", err
	}
	expires := j.expires
	return oscal.Observation{
		UUID:        obs.UUID.String(),
		Title:       j.control.ID + " on " + b.Short,
		Description: "Assessment of " + j.control.ID + " against " + b.Name + " for " + Period + ".",
		Methods:     []string{j.method},
		Types:       ptr([]string{"control-objective"}),
		Collected:   j.collected,
		// The native expires field drives every countdown in the HUD;
		// nothing in the namespace props duplicates it.
		Expires: &expires,
		Origins: &origin,
		// The control an observation assesses, stated rather than
		// implied: OSCAL binds observations to controls through findings,
		// and an observation that never produced one would otherwise have
		// no path back to the control its key is derived from.
		Links: &[]oscal.Link{{
			Rel:              "assessed-control",
			Href:             "#" + j.requirement,
			ResourceFragment: j.control.ID,
			Text:             "Implemented requirement for " + j.control.ID,
		}},
		Subjects: &[]oscal.SubjectReference{{
			Type:        "component",
			SubjectUuid: j.component,
		}},
		RelevantEvidence: &[]oscal.RelevantEvidence{{
			Href:        "#" + evidenceUUID[j.evidence.path],
			Description: j.evidence.title,
		}},
	}, obs.UUID.String(), nil
}

// attestation is the ISO's sign-off for a satisfied manual control, recorded
// where OSCAL puts it. The part carries the attestation key so the claim has
// an identity that federates. ok is false for a control that is not manual.
func (g *Generator) attestation(b Boundary, j judgement, ssp string) (oscal.AssessmentPart, bool, error) {
	if !manualControls[j.control.ID] {
		return oscal.AssessmentPart{}, false, nil
	}
	att, err := g.keys.Attestation(ssp, j.source, j.control.ID, j.component, Period)
	if err != nil {
		return oscal.AssessmentPart{}, false, err
	}
	return oscal.AssessmentPart{
		UUID:  att.UUID.String(),
		Name:  "asset",
		Class: j.control.ID,
		Title: "Attestation for " + j.control.ID,
		Prose: "The Information System Security Officer attests that " + j.control.ID + " is implemented for " + b.Name + " in " + Period + ".",
	}, true, nil
}

// shortfall is everything an unsatisfied judgement produces: the finding, its
// risk as the assessment results state it, the same risk with its remediation
// plan as the POA&M states it, and the POA&M item.
type shortfall struct {
	finding  oscal.Finding
	risk     oscal.Risk
	poamRisk oscal.Risk
	item     oscal.PoamItem
}

func (g *Generator) shortfall(b Boundary, j judgement, ssp, iso, ao string, origin []oscal.Origin, obsUUID string) (shortfall, error) {
	finding, err := g.keys.Finding(ssp, j.source, j.control.ID, j.component, Period)
	if err != nil {
		return shortfall{}, err
	}
	risk, err := g.keys.Risk(ssp, j.source, j.control.ID, j.component, Period)
	if err != nil {
		return shortfall{}, err
	}
	item, err := g.keys.POAMItem(ssp, j.source, j.control.ID, j.component, Period)
	if err != nil {
		return shortfall{}, err
	}
	related := &[]oscal.RelatedObservation{{ObservationUuid: obsUUID}}

	out := shortfall{finding: oscal.Finding{
		UUID:                finding.UUID.String(),
		Title:               j.control.ID + " is not satisfied on " + b.Short,
		Description:         "The assessment of " + j.control.ID + " did not satisfy the control objective.",
		Origins:             &origin,
		RelatedObservations: related,
		RelatedRisks:        &[]oscal.AssociatedRisk{{RiskUuid: risk.UUID.String()}},
		Target: oscal.FindingTarget{
			Type:     "objective-id",
			TargetId: j.control.ID + "_obj",
			Title:    j.control.Title,
			Status:   oscal.ObjectiveStatus{State: "not-satisfied"},
		},
	}}

	props := []oscal.Property{sparcProp(PropBlocksATO, boolValue(j.blocksATO))}
	status := "open"
	var riskLog *oscal.RiskLog
	if j.accepted {
		status = "deviation-approved"
		accepted, log, err := g.acceptance(ssp, risk.UUID.String(), ao, status)
		if err != nil {
			return shortfall{}, err
		}
		props = append(props, accepted...)
		riskLog = log
	}

	out.risk = oscal.Risk{
		UUID:                risk.UUID.String(),
		Title:               j.control.ID + " shortfall on " + b.Short,
		Description:         "Residual risk from the " + j.control.ID + " finding.",
		Statement:           "Until " + j.control.ID + " is satisfied, " + b.Name + " carries this exposure.",
		Status:              status,
		Props:               &props,
		Origins:             &origin,
		RelatedObservations: related,
		RiskLog:             riskLog,
	}

	// The POA&M re-states the risk and is where its milestones live. The
	// UUID is the same object in both documents, which is what makes
	// deduplicating on it safe. Drawn after the acceptance's condition, as
	// the committed fixtures expect.
	deadline := days(45 + g.rng.intn(120))
	out.poamRisk = out.risk
	out.poamRisk.Deadline = &deadline
	out.poamRisk.Remediations = &[]oscal.Response{{
		UUID:        g.objectUUID("remediation", b.Slug, j.control.ID),
		Lifecycle:   "planned",
		Title:       "Remediate " + j.control.ID,
		Description: "Planned remediation for " + j.control.ID + " on " + b.Name + ".",
		Tasks: &[]oscal.Task{{
			UUID:        g.objectUUID("milestone", b.Slug, j.control.ID),
			Type:        "milestone",
			Title:       "Milestone: " + j.control.ID + " remediation complete",
			Description: "The milestone the projection engine reads as open before its date.",
			Timing:      &oscal.EventTiming{OnDate: &oscal.OnDateCondition{Date: deadline}},
		}},
	}}

	out.item = oscal.PoamItem{
		UUID:                item.UUID.String(),
		Title:               j.control.ID + " remediation",
		Description:         "Tracked remediation of the " + j.control.ID + " finding on " + b.Name + ".",
		RelatedFindings:     &[]oscal.RelatedFinding{{FindingUuid: finding.UUID.String()}},
		RelatedRisks:        &[]oscal.AssociatedRisk{{RiskUuid: risk.UUID.String()}},
		RelatedObservations: related,
		Origins: &[]oscal.PoamItemOrigin{{Actors: []oscal.OriginActor{{
			Type:      "party",
			ActorUuid: iso,
			RoleId:    roleISO,
		}}}},
	}
	return out, nil
}

// acceptance is what an accepted risk carries: the condition that reopens it,
// so the acceptance expires by itself rather than by someone remembering, and
// the AO's decision in the risk log.
func (g *Generator) acceptance(ssp, riskUUID, ao, status string) ([]oscal.Property, *oscal.RiskLog, error) {
	props := []oscal.Property{
		sparcProp(PropConditionExpires, days(90+g.rng.intn(120)).Format(dateLayout)),
		sparcProp(PropTrigger, "score<0.85"),
	}
	// Keyed on the day the decision was taken, not the quarter it fell in:
	// an AO decision happens on a date, and the value has to line up with
	// the next-decision-date prop.
	decided := days(-7)
	decision, err := g.keys.AODecision(ssp, riskUUID, decided.Format(dateLayout))
	if err != nil {
		return nil, nil, err
	}
	return props, &oscal.RiskLog{Entries: []oscal.RiskLogEntry{{
		UUID:         decision.UUID.String(),
		Title:        "Risk accepted with conditions",
		Description:  "The Authorizing Official accepted this risk for " + Period + ", subject to the recorded condition.",
		Start:        decided,
		StatusChange: status,
		LoggedBy: &[]oscal.LoggedBy{{
			PartyUuid: ao,
			RoleId:    roleAO,
		}},
	}}}, nil
}

// assessmentMetadata is the metadata the assessment results and the POA&M
// share: the boundary's parties, the fixture roles and the responsible parties.
func (g *Generator) assessmentMetadata(b Boundary, title string) oscal.Metadata {
	meta := g.baseMetadata(title)
	parties := g.parties(b)
	meta.Parties = &parties
	meta.Roles = &fixtureRoles
	responsible := g.responsibleParties(b)
	meta.ResponsibleParties = &responsible
	return meta
}

func boolValue(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// assessmentPlan is the plan the results import, and the document that names
// the SSP they assess: import-ap, then import-ssp, is how a reader places an
// assessment-results document (#120).
func (g *Generator) assessmentPlan(b Boundary, ssp string, reviewed oscal.ReviewedControls) oscal.OscalCompleteSchema {
	return oscal.OscalCompleteSchema{AssessmentPlan: &oscal.AssessmentPlan{
		UUID:             g.documentUUID("assessment-plan", b.Slug),
		Metadata:         g.assessmentMetadata(b, b.Name+" continuous monitoring plan "+Period),
		ImportSsp:        oscal.ImportSsp{Href: "#" + ssp},
		ReviewedControls: reviewed,
	}}
}

// satisfiedFinding records that a control met its objective.
func (g *Generator) satisfiedFinding(b Boundary, j judgement, ssp string, origin []oscal.Origin, obsUUID string) (oscal.Finding, error) {
	finding, err := g.keys.Finding(ssp, j.source, j.control.ID, j.component, Period)
	if err != nil {
		return oscal.Finding{}, err
	}
	return oscal.Finding{
		UUID:                finding.UUID.String(),
		Title:               j.control.ID + " is satisfied on " + b.Short,
		Description:         "The assessment of " + j.control.ID + " satisfied the control objective.",
		Origins:             &origin,
		RelatedObservations: &[]oscal.RelatedObservation{{ObservationUuid: obsUUID}},
		Target: oscal.FindingTarget{
			Type:     "objective-id",
			TargetId: j.control.ID + "_obj",
			Title:    j.control.Title,
			Status:   oscal.ObjectiveStatus{State: "satisfied"},
		},
	}, nil
}

// satisfied is what a satisfied control emits: a satisfied finding, which is
// how OSCAL ties an observation to the control it assessed and the only path a
// reader that is not this generator has (#120), and the ISO's attestation for
// a manual control. No draw.
func (g *Generator) satisfied(b Boundary, j judgement, ssp string, origin []oscal.Origin, obsUUID string) (oscal.Finding, *oscal.AssessmentPart, error) {
	finding, err := g.satisfiedFinding(b, j, ssp, origin, obsUUID)
	if err != nil {
		return oscal.Finding{}, nil, err
	}
	part, ok, err := g.attestation(b, j, ssp)
	if err != nil || !ok {
		return finding, nil, err
	}
	return finding, &part, nil
}
