package fixtures

import (
	"strconv"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"

	"github.com/risk-sentinel/sparc-horizon/internal/canonical"
	"github.com/risk-sentinel/sparc-horizon/internal/keys"
)

// One SSP per authorization boundary, which is where three of the four tiers
// meet: the federation and organization parties in metadata, the boundary in
// the SSP itself, and the systems as components and inventory items. Because
// the same organization party UUID appears in every SSP beneath it, the tree
// is a join rather than a hierarchy table.

const (
	roleAO  = "authorizing-official"
	roleSO  = "system-owner"
	roleISO = "information-system-security-officer"
)

var fixtureRoles = []oscal.Role{
	{ID: roleAO, Title: "Authorizing Official"},
	{ID: roleSO, Title: "System Owner"},
	{ID: roleISO, Title: "Information System Security Officer"},
}

func (g *Generator) sspUUID(b Boundary) string { return g.documentUUID("ssp", b.Slug) }

func (g *Generator) federationPartyUUID() string {
	return g.objectUUID("party", "federation", FederationSlug)
}

func (g *Generator) orgPartyUUID(o Org) string {
	return g.objectUUID("party", "organization", o.Slug)
}

// partyUUID for a role holder. The fixtures name role holders by their role
// rather than inventing people: a fixture that reads like personal data
// eventually gets treated as some.
func (g *Generator) rolePartyUUID(b Boundary, role string) string {
	return g.objectUUID("party", "person", b.Slug, role)
}

func (g *Generator) componentUUID(b Boundary, systemSlug string) string {
	return g.objectUUID("component", b.Slug, systemSlug)
}

func (g *Generator) inventoryUUID(b Boundary, systemSlug string) string {
	return g.objectUUID("inventory-item", b.Slug, systemSlug)
}

// profileSource is what `source-uuid` resolves to for a control declared in an
// SSP: the SSP carries no `source` of its own on its control-implementation,
// so the fallback in docs/03-data-model.md applies — the SSP's
// `import-profile`, resolved to the back-matter resource it names.
func (g *Generator) profileSource() keys.Source {
	return keys.Source{UUID: g.profileUUID(), Vocabulary: canonical.VocabNIST80053}
}

// securityHubSource is the other authority, reaching the boundary through the
// inherited platform component's own component definition.
func (g *Generator) securityHubSource() keys.Source {
	return keys.Source{UUID: g.securityHubCatalogUUID(), Vocabulary: canonical.VocabOpaque}
}

func providerBoundary() Boundary {
	for _, b := range Boundaries {
		if b.Slug == ProviderBoundarySlug {
			return b
		}
	}
	return Boundary{}
}

// exportedHalf is the provider's side of an inherited control.
type exportedHalf struct {
	ControlID          string
	ProvidedUUID       string
	ResponsibilityUUID string
	ComponentUUID      string
}

// exportedResponsibilities derives the provider's exports once, before any SSP
// is built, because every consumer answers the same responsibility UUID.
func (g *Generator) exportedResponsibilities() (map[string]exportedHalf, error) {
	provider := providerBoundary()
	component := g.componentUUID(provider, provider.Systems[0].Slug)
	out := map[string]exportedHalf{}

	for _, controlID := range InheritedControls {
		half, err := g.keys.Responsibility(g.sspUUID(provider), g.profileSource(), controlID, component, keys.HalfProvider)
		if err != nil {
			return nil, err
		}
		out[controlID] = exportedHalf{
			ControlID:          controlID,
			ProvidedUUID:       g.objectUUID("provided", provider.Slug, controlID),
			ResponsibilityUUID: half.UUID.String(),
			ComponentUUID:      component,
		}
	}
	return out, nil
}

func (g *Generator) parties(b Boundary) []oscal.Party {
	org := b.Org()
	federation := g.federationPartyUUID()
	orgParty := g.orgPartyUUID(org)

	parties := []oscal.Party{
		{
			UUID:      federation,
			Type:      "organization",
			Name:      FederationName,
			ShortName: FederationShort,
			Props:     &[]oscal.Property{sparcProp(PropNodeType, "federation")},
			Remarks:   "The federation tier. Its signed manifest lives in SPARC's trust fabric, not in this document.",
		},
		{
			UUID:                  orgParty,
			Type:                  "organization",
			Name:                  org.Name,
			ShortName:             org.Short,
			MemberOfOrganizations: &[]string{federation},
			Props:                 &[]oscal.Property{sparcProp(PropNodeType, "organization")},
		},
	}
	for _, r := range fixtureRoles {
		title := ""
		for _, role := range fixtureRoles {
			if role.ID == r.ID {
				title = role.Title
			}
		}
		parties = append(parties, oscal.Party{
			UUID:                  g.rolePartyUUID(b, r.ID),
			Type:                  "person",
			Name:                  b.Short + " " + title,
			MemberOfOrganizations: &[]string{orgParty},
		})
	}
	return parties
}

func (g *Generator) responsibleParties(b Boundary) []oscal.ResponsibleParty {
	out := make([]oscal.ResponsibleParty, 0, len(fixtureRoles))
	for _, r := range fixtureRoles {
		out = append(out, oscal.ResponsibleParty{
			RoleId:     r.ID,
			PartyUuids: []string{g.rolePartyUUID(b, r.ID)},
		})
	}
	return out
}

func (g *Generator) components(b Boundary) []oscal.SystemComponent {
	out := make([]oscal.SystemComponent, 0, len(b.Systems)+1)
	for _, s := range b.Systems {
		out = append(out, oscal.SystemComponent{
			UUID:        g.componentUUID(b, s.Slug),
			Type:        s.Type,
			Title:       s.Name,
			Description: s.Purpose,
			Purpose:     s.Purpose,
			Status:      oscal.SystemComponentStatus{State: "operational"},
			ResponsibleRoles: &[]oscal.ResponsibleRole{{
				RoleId:     roleSO,
				PartyUuids: &[]string{g.rolePartyUUID(b, roleSO)},
			}},
		})
	}
	// The inherited platform component. Its props come from an authority
	// Horizon does not own and are re-emitted exactly as issued; the third has
	// no `ns` at all, which OSCAL reads as the default NIST namespace.
	out = append(out, oscal.SystemComponent{
		UUID:        g.componentUUID(b, InheritedComponentSlug),
		Type:        "service",
		Title:       InheritedComponentName,
		Description: "Inherited platform service, assessed by the provider against AWS Security Hub controls.",
		Status:      oscal.SystemComponentStatus{State: "operational"},
		Props: &[]oscal.Property{
			{Name: "EvaluatedServices", Ns: NamespaceAWS, Value: "acm"},
			{Name: "SeverityLabel", Ns: NamespaceAWS, Value: "MEDIUM"},
			{Name: "implementation-point", Value: "external"},
		},
	})
	return out
}

func (g *Generator) inventory(b Boundary) []oscal.InventoryItem {
	out := make([]oscal.InventoryItem, 0, len(b.Systems))
	for _, s := range b.Systems {
		out = append(out, oscal.InventoryItem{
			UUID:        g.inventoryUUID(b, s.Slug),
			Description: s.Name + " inventory record.",
			// asset-id carries no `ns`, and is what an HDF target matches on.
			Props: &[]oscal.Property{{Name: "asset-id", Value: b.Slug + "/" + s.Slug}},
			ImplementedComponents: &[]oscal.ImplementedComponent{{
				ComponentUuid: g.componentUUID(b, s.Slug),
			}},
		})
	}
	return out
}

// implementedRequirements maps the profile's controls onto components. Each
// control lands on two components, chosen by position so the mapping is
// deterministic and spread rather than random.
func (g *Generator) implementedRequirements(b Boundary, exports map[string]exportedHalf) []oscal.ImplementedRequirement {
	out := make([]oscal.ImplementedRequirement, 0, len(NISTControls))
	isProvider := b.Slug == ProviderBoundarySlug

	for i, c := range NISTControls {
		systems := []System{b.Systems[i%len(b.Systems)]}
		if len(b.Systems) > 1 {
			systems = append(systems, b.Systems[(i+1)%len(b.Systems)])
		}

		byComponents := make([]oscal.ByComponent, 0, len(systems))
		for j, s := range systems {
			bc := oscal.ByComponent{
				UUID:                 g.objectUUID("by-component", b.Slug, c.ID, s.Slug),
				ComponentUuid:        g.componentUUID(b, s.Slug),
				Description:          s.Name + " implements " + c.ID + ".",
				ImplementationStatus: &oscal.ImplementationStatus{State: "implemented"},
			}
			export, inherited := exports[c.ID]
			if inherited && j == 0 {
				if isProvider {
					// The provider declares what it provides and what it
					// leaves to the consumer.
					bc.ComponentUuid = export.ComponentUUID
					bc.Export = &oscal.Export{
						Description: "Provided to every leveraging boundary in the federation.",
						Provided: &[]oscal.ProvidedControlImplementation{{
							UUID:        export.ProvidedUUID,
							Description: "Authentication and authenticator management for leveraging boundaries.",
						}},
						Responsibilities: &[]oscal.ControlImplementationResponsibility{{
							UUID:         export.ResponsibilityUUID,
							ProvidedUuid: export.ProvidedUUID,
							Description:  "The leveraging boundary configures its own session and account policy against this service.",
						}},
					}
				} else {
					// The consumer's half. A hybrid control is green only when
					// both halves have unexpired observations.
					satisfied, err := g.keys.Responsibility(g.sspUUID(b), g.profileSource(), c.ID, g.componentUUID(b, s.Slug), keys.HalfConsumer)
					if err == nil {
						bc.Inherited = &[]oscal.InheritedControlImplementation{{
							UUID:         g.objectUUID("inherited", b.Slug, c.ID),
							ProvidedUuid: export.ProvidedUUID,
							Description:  "Inherited from " + providerBoundary().Name + ".",
						}}
						bc.Satisfied = &[]oscal.SatisfiedControlImplementationResponsibility{{
							UUID:               satisfied.UUID.String(),
							ResponsibilityUuid: export.ResponsibilityUUID,
							Description:        "Boundary-side configuration answering the provider's stated responsibility.",
						}}
					}
				}
			}
			byComponents = append(byComponents, bc)
		}

		out = append(out, oscal.ImplementedRequirement{
			UUID:         g.objectUUID("implemented-requirement", b.Slug, c.ID),
			ControlId:    c.ID,
			ByComponents: &byComponents,
			ResponsibleRoles: &[]oscal.ResponsibleRole{{
				RoleId:     roleISO,
				PartyUuids: &[]string{g.rolePartyUUID(b, roleISO)},
			}},
		})
	}
	return out
}

func (g *Generator) ssp(b Boundary, profileBytes, providerSSPBytes []byte, exports map[string]exportedHalf) oscal.OscalCompleteSchema {
	org := b.Org()

	meta := g.baseMetadata(b.Name + " system security plan")
	parties := g.parties(b)
	meta.Parties = &parties
	meta.Roles = &fixtureRoles
	responsible := g.responsibleParties(b)
	meta.ResponsibleParties = &responsible
	meta.Props = &[]oscal.Property{
		sparcProp(PropNodeType, "boundary"),
		sparcProp(PropParentUUID, g.orgPartyUUID(org)),
		sparcProp(PropNextDecisionDate, b.NextDecision),
		sparcProp(PropFIPS199, b.FIPS),
	}

	resources := []oscal.Resource{
		resourceFor(g.profileUUID(), "Fixture moderate baseline", "./profile-moderate.json", sha256Hex(profileBytes)),
	}

	implementation := oscal.SystemImplementation{
		Components:     g.components(b),
		InventoryItems: opt(g.inventory(b)),
	}
	if b.Slug != ProviderBoundarySlug && providerSSPBytes != nil {
		provider := providerBoundary()
		resources = append(resources, resourceFor(g.sspUUID(provider), provider.Name+" system security plan", "./ssp-"+provider.Slug+".json", sha256Hex(providerSSPBytes)))
		implementation.LeveragedAuthorizations = &[]oscal.LeveragedAuthorization{{
			UUID:           g.objectUUID("leveraged-authorization", b.Slug, provider.Slug),
			Title:          provider.Name,
			PartyUuid:      g.orgPartyUUID(provider.Org()),
			DateAuthorized: "2026-03-02",
			Links: &[]oscal.Link{{
				Href: "#" + g.sspUUID(provider),
				Rel:  "leveraged-authorization",
			}},
			Remarks: "The reverse-inheritance index behind blast radius is built from this link.",
		}}
	}

	impact := "fips-199-" + b.FIPS
	characteristics := oscal.SystemCharacteristics{
		SystemName:      b.Name,
		SystemNameShort: b.Short,
		Description:     b.Description,
		// Both addressing schemes on one object: SPARC addresses this
		// boundary by slug and by numeric id, Horizon joins on the SSP UUID.
		SystemIds: []oscal.SystemId{
			{ID: b.Slug, IdentifierType: NamespaceSPARC + "/slug"},
			{ID: strconv.Itoa(b.ID), IdentifierType: NamespaceSPARC + "/id"},
		},
		SecuritySensitivityLevel: b.FIPS,
		SecurityImpactLevel: &oscal.SecurityImpactLevel{
			SecurityObjectiveConfidentiality: b.FIPS,
			SecurityObjectiveIntegrity:       b.FIPS,
			SecurityObjectiveAvailability:    b.FIPS,
		},
		SystemInformation: oscal.SystemInformation{InformationTypes: []oscal.InformationType{{
			UUID:                  g.objectUUID("information-type", b.Slug),
			Title:                 b.Name + " operational records",
			Description:           "Synthetic information type for the fixture federation.",
			ConfidentialityImpact: &oscal.Impact{Base: impact},
			IntegrityImpact:       &oscal.Impact{Base: impact},
			AvailabilityImpact:    &oscal.Impact{Base: impact},
		}}},
		Status:                oscal.Status{State: "operational"},
		AuthorizationBoundary: oscal.AuthorizationBoundary{Description: b.Description},
		DateAuthorized:        "2026-03-02",
	}

	return oscal.OscalCompleteSchema{SystemSecurityPlan: &oscal.SystemSecurityPlan{
		UUID:                  g.sspUUID(b),
		Metadata:              meta,
		ImportProfile:         oscal.ImportProfile{Href: "#" + g.profileUUID()},
		SystemCharacteristics: characteristics,
		SystemImplementation:  implementation,
		ControlImplementation: oscal.ControlImplementation{
			Description:             "Controls resolved from the fixture moderate baseline. The control-implementation carries no `source` of its own, so `source-uuid` resolves through import-profile.",
			ImplementedRequirements: g.implementedRequirements(b, exports),
		},
		BackMatter: &oscal.BackMatter{Resources: &resources},
	}}
}

// ptr is the shape go-oscal wants for its optional fields.
func ptr[T any](v T) *T { return &v }

// opt is ptr for a slice that may be empty. A pointer to a nil slice marshals
// to null, and an OSCAL field set to null is not an omitted field — it is an
// invalid one.
func opt[T any](s []T) *[]T {
	if len(s) == 0 {
		return nil
	}
	return &s
}
