package fixtures

import (
	"strings"
	"time"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"

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

// assessed is one boundary's assessment-results and POA&M documents.
type assessed struct {
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

	// One back-matter resource per evidence artifact, keyed on the hash of its
	// content: the same bytes submitted twice are the same resource.
	resources := make([]oscal.Resource, 0, len(artifacts)+2)
	evidenceUUID := map[string]string{}
	for _, a := range artifacts {
		key, err := g.keys.EvidenceResource(ssp, a.digest)
		if err != nil {
			return assessed{}, err
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
	resources = append(resources,
		resourceFor(ssp, b.Name+" system security plan", "./ssp-"+b.Slug+".json", sha256Hex(sspBytes)),
		oscal.Resource{
			UUID:        g.objectUUID("assessment-plan-reference", b.Slug),
			Title:       b.Name + " continuous monitoring plan",
			Description: "Cited rather than modelled: these fixtures carry no assessment-plan document.",
			Citation:    &oscal.Citation{Text: b.Name + " continuous monitoring plan, " + Period},
		},
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

	for _, j := range g.judgements(b, artifacts) {
		assessedIDs = append(assessedIDs, j.control.ID)

		obs, err := g.keys.Observation(ssp, j.source, j.control.ID, j.component, Period)
		if err != nil {
			return assessed{}, err
		}
		expires := j.expires
		observations = append(observations, oscal.Observation{
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
		})

		if j.satisfied {
			if manualControls[j.control.ID] {
				// The ISO's sign-off, recorded where OSCAL puts it. The part
				// carries the attestation key so the claim has an identity
				// that federates.
				att, err := g.keys.Attestation(ssp, j.source, j.control.ID, j.component, Period)
				if err != nil {
					return assessed{}, err
				}
				attested = append(attested, oscal.AssessmentPart{
					UUID:  att.UUID.String(),
					Name:  "asset",
					Class: j.control.ID,
					Title: "Attestation for " + j.control.ID,
					Prose: "The Information System Security Officer attests that " + j.control.ID + " is implemented for " + b.Name + " in " + Period + ".",
				})
			}
			continue
		}

		finding, err := g.keys.Finding(ssp, j.source, j.control.ID, j.component, Period)
		if err != nil {
			return assessed{}, err
		}
		risk, err := g.keys.Risk(ssp, j.source, j.control.ID, j.component, Period)
		if err != nil {
			return assessed{}, err
		}
		item, err := g.keys.POAMItem(ssp, j.source, j.control.ID, j.component, Period)
		if err != nil {
			return assessed{}, err
		}

		findings = append(findings, oscal.Finding{
			UUID:                finding.UUID.String(),
			Title:               j.control.ID + " is not satisfied on " + b.Short,
			Description:         "The assessment of " + j.control.ID + " did not satisfy the control objective.",
			Origins:             &origin,
			RelatedObservations: &[]oscal.RelatedObservation{{ObservationUuid: obs.UUID.String()}},
			RelatedRisks:        &[]oscal.AssociatedRisk{{RiskUuid: risk.UUID.String()}},
			Target: oscal.FindingTarget{
				Type:     "objective-id",
				TargetId: strings.ReplaceAll(j.control.ID, ".", "_") + "_obj",
				Title:    j.control.Title,
				Status:   oscal.ObjectiveStatus{State: "not-satisfied"},
			},
		})

		props := []oscal.Property{sparcProp(PropBlocksATO, boolValue(j.blocksATO))}
		status := "open"
		var riskLog *oscal.RiskLog
		if j.accepted {
			// An accepted risk carries the condition that reopens it, so the
			// acceptance expires by itself rather than by someone remembering.
			status = "deviation-approved"
			props = append(props,
				sparcProp(PropConditionExpires, days(90+g.rng.intn(120)).Format("2006-01-02")),
				sparcProp(PropTrigger, "score<0.85"),
			)
			decision, err := g.keys.AODecision(ssp, risk.UUID.String(), Period)
			if err != nil {
				return assessed{}, err
			}
			riskLog = &oscal.RiskLog{Entries: []oscal.RiskLogEntry{{
				UUID:         decision.UUID.String(),
				Title:        "Risk accepted with conditions",
				Description:  "The Authorizing Official accepted this risk for " + Period + ", subject to the recorded condition.",
				Start:        days(-7),
				StatusChange: status,
				LoggedBy: &[]oscal.LoggedBy{{
					PartyUuid: ao,
					RoleId:    roleAO,
				}},
			}}}
		}

		base := oscal.Risk{
			UUID:                risk.UUID.String(),
			Title:               j.control.ID + " shortfall on " + b.Short,
			Description:         "Residual risk from the " + j.control.ID + " finding.",
			Statement:           "Until " + j.control.ID + " is satisfied, " + b.Name + " carries this exposure.",
			Status:              status,
			Props:               &props,
			Origins:             &origin,
			RelatedObservations: &[]oscal.RelatedObservation{{ObservationUuid: obs.UUID.String()}},
			RiskLog:             riskLog,
		}
		risks = append(risks, base)

		// The POA&M re-states the risk and is where its milestones live. The
		// UUID is the same object in both documents, which is what makes
		// deduplicating on it safe.
		deadline := days(45 + g.rng.intn(120))
		withPlan := base
		withPlan.Deadline = &deadline
		withPlan.Remediations = &[]oscal.Response{{
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
		poamRisks = append(poamRisks, withPlan)

		poamItems = append(poamItems, oscal.PoamItem{
			UUID:                item.UUID.String(),
			Title:               j.control.ID + " remediation",
			Description:         "Tracked remediation of the " + j.control.ID + " finding on " + b.Name + ".",
			RelatedFindings:     &[]oscal.RelatedFinding{{FindingUuid: finding.UUID.String()}},
			RelatedRisks:        &[]oscal.AssociatedRisk{{RiskUuid: risk.UUID.String()}},
			RelatedObservations: &[]oscal.RelatedObservation{{ObservationUuid: obs.UUID.String()}},
			Origins: &[]oscal.PoamItemOrigin{{Actors: []oscal.OriginActor{{
				Type:      "party",
				ActorUuid: iso,
				RoleId:    roleISO,
			}}}},
		})
	}

	selections := make([]oscal.AssessedControlsSelectControlById, 0, len(assessedIDs))
	for _, id := range assessedIDs {
		selections = append(selections, oscal.AssessedControlsSelectControlById{ControlId: id})
	}
	reviewed := oscal.ReviewedControls{ControlSelections: []oscal.AssessedControls{{
		Description:     "Controls assessed for " + b.Name + " in " + Period + ".",
		IncludeControls: &selections,
	}}}

	result := oscal.Result{
		UUID:             g.objectUUID("result", b.Slug, Period),
		Title:            b.Name + " assessment " + Period,
		Description:      "Continuous monitoring results for " + b.Name + ".",
		Start:            days(-21),
		End:              ptr(days(-1)),
		ReviewedControls: reviewed,
		Observations:     opt(observations),
		Findings:         opt(findings),
		Risks:            opt(risks),
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

	arMeta := g.baseMetadata(b.Name + " assessment results " + Period)
	arParties := g.parties(b)
	arMeta.Parties = &arParties
	arMeta.Roles = &fixtureRoles
	arResponsible := g.responsibleParties(b)
	arMeta.ResponsibleParties = &arResponsible

	poamMeta := g.baseMetadata(b.Name + " plan of action and milestones " + Period)
	poamParties := g.parties(b)
	poamMeta.Parties = &poamParties
	poamMeta.Roles = &fixtureRoles
	poamResponsible := g.responsibleParties(b)
	poamMeta.ResponsibleParties = &poamResponsible

	arResources := resources
	return assessed{
		results: oscal.OscalCompleteSchema{AssessmentResults: &oscal.AssessmentResults{
			UUID:       g.documentUUID("assessment-results", b.Slug),
			Metadata:   arMeta,
			ImportAp:   oscal.ImportAp{Href: "#" + g.objectUUID("assessment-plan-reference", b.Slug)},
			Results:    []oscal.Result{result},
			BackMatter: &oscal.BackMatter{Resources: &arResources},
		}},
		poam: oscal.OscalCompleteSchema{PlanOfActionAndMilestones: &oscal.PlanOfActionAndMilestones{
			UUID:      g.documentUUID("poam", b.Slug),
			Metadata:  poamMeta,
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

func boolValue(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
