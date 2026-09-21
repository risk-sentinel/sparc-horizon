package fixtures

import (
	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"
)

// The two catalogs and the profile over one of them.
//
// Two authorities are in the fixtures on purpose. A control identifier is
// unique only within the catalog that defines it, and `source-uuid` is what
// partitions the key space by authority — a rule that cannot be exercised
// against a federation where every identifier happens to be NIST's.

// sparcProp builds a prop in the namespace Horizon owns.
func sparcProp(name, value string) oscal.Property {
	return oscal.Property{Name: name, Ns: NamespaceSPARC, Value: value}
}

func (g *Generator) baseMetadata(title string) oscal.Metadata {
	published := BaseDate
	return oscal.Metadata{
		Title:        title,
		LastModified: BaseDate,
		Published:    &published,
		Version:      DocumentVersion,
		OscalVersion: OSCALVersion,
	}
}

// resourceFor builds the back-matter resource that names another document in
// the tree.
//
// Its UUID is the referenced document's own UUID rather than a fresh one.
// docs/03-data-model.md resolves `source-uuid` to the UUID of the back-matter
// resource a `source` names, and a per-document resource UUID would give the
// same catalog a different qualifier in every SSP that cites it — which is
// exactly the federation-wide dedup the qualifier exists to make possible.
func resourceFor(documentUUID, title, href, digest string) oscal.Resource {
	return oscal.Resource{
		UUID:        documentUUID,
		Title:       title,
		Description: "Referenced document; the resource UUID is the referenced document's own UUID so the reference is stable across every document that makes it.",
		Rlinks: &[]oscal.ResourceLink{{
			Href:      href,
			MediaType: "application/oscal+json",
			Hashes:    &[]oscal.Hash{{Algorithm: "SHA-256", Value: digest}},
		}},
	}
}

func (g *Generator) nistCatalogUUID() string {
	return g.documentUUID("catalog", "nist-800-53-subset")
}

func (g *Generator) securityHubCatalogUUID() string {
	return g.documentUUID("catalog", "aws-security-hub-subset")
}

func (g *Generator) profileUUID() string {
	return g.documentUUID("profile", "moderate")
}

func (g *Generator) nistCatalog() oscal.OscalCompleteSchema {
	groups := make([]oscal.Group, 0, len(NISTFamilies))
	for _, fam := range NISTFamilies {
		controls := make([]oscal.Control, 0, 2)
		for _, c := range NISTControls {
			if c.Family != fam.ID {
				continue
			}
			controls = append(controls, oscal.Control{
				ID:    c.ID,
				Class: "SP800-53",
				Title: c.Title,
				Parts: &[]oscal.Part{{
					ID:    c.ID + "_smt",
					Name:  "statement",
					Prose: "Synthetic statement for " + c.ID + ". These fixtures carry control identifiers and structure, not NIST's control text.",
				}},
			})
		}
		groups = append(groups, oscal.Group{
			ID:       fam.ID,
			Class:    "family",
			Title:    fam.Title,
			Controls: &controls,
		})
	}

	meta := g.baseMetadata("Fixture subset of NIST SP 800-53 Revision 5")
	meta.Remarks = "Twelve controls across six families, for the SPARC Horizon fixture federation. Not a substitute for the published catalog."

	return oscal.OscalCompleteSchema{Catalog: &oscal.Catalog{
		UUID:     g.nistCatalogUUID(),
		Metadata: meta,
		Groups:   &groups,
	}}
}

func (g *Generator) securityHubCatalog() oscal.OscalCompleteSchema {
	seen := map[string]bool{}
	groups := make([]oscal.Group, 0, 4)
	for _, c := range SecurityHubControls {
		if seen[c.Family] {
			continue
		}
		seen[c.Family] = true
		controls := make([]oscal.Control, 0, 1)
		for _, d := range SecurityHubControls {
			if d.Family != c.Family {
				continue
			}
			controls = append(controls, oscal.Control{
				ID:    d.ID,
				Class: "AWS-SecurityHub",
				Title: d.Title,
			})
		}
		groups = append(groups, oscal.Group{
			ID:       c.Family,
			Class:    "service",
			Title:    "AWS Security Hub controls for " + c.Family,
			Controls: &controls,
		})
	}

	meta := g.baseMetadata("Fixture subset of AWS Security Hub controls")
	meta.Remarks = "A second catalog authority. Its identifiers are carried exactly as issued: ACM.1 is not acm.1, and normalising it into NIST's form would produce an identifier that validates and names nothing."

	return oscal.OscalCompleteSchema{Catalog: &oscal.Catalog{
		UUID:     g.securityHubCatalogUUID(),
		Metadata: meta,
		Groups:   &groups,
	}}
}

func (g *Generator) profile(catalogBytes []byte) oscal.OscalCompleteSchema {
	ids := make([]string, 0, len(NISTControls))
	for _, c := range NISTControls {
		ids = append(ids, c.ID)
	}

	meta := g.baseMetadata("Fixture moderate baseline")
	meta.Remarks = "The resolved control set every fixture SSP imports. Where a control-implementation carries no source of its own, this profile is what `source-uuid` resolves to."

	return oscal.OscalCompleteSchema{Profile: &oscal.Profile{
		UUID:     g.profileUUID(),
		Metadata: meta,
		Imports: []oscal.Import{{
			Href:            "#" + g.nistCatalogUUID(),
			IncludeControls: &[]oscal.SelectControlById{{WithIds: &ids}},
		}},
		BackMatter: &oscal.BackMatter{Resources: &[]oscal.Resource{
			resourceFor(g.nistCatalogUUID(), "Fixture subset of NIST SP 800-53 Revision 5", "./catalog-nist-800-53-subset.json", sha256Hex(catalogBytes)),
		}},
	}}
}

// awsComponentDefinition is how the second authority reaches a boundary:
// inherited AWS service components carry their own control-implementations,
// sourced to the Security Hub catalog rather than to the SSP's profile. It is
// the rule-1 path in docs/03-data-model.md — `source` on the
// control-implementation — where the SSPs exercise the rule-2 fallback.
func (g *Generator) awsComponentDefinition(securityHubBytes []byte) oscal.OscalCompleteSchema {
	reqs := make([]oscal.ImplementedRequirementControlImplementation, 0, len(SecurityHubControls))
	for _, c := range SecurityHubControls {
		reqs = append(reqs, oscal.ImplementedRequirementControlImplementation{
			UUID:        g.objectUUID("cdef-requirement", InheritedComponentSlug, c.ID),
			ControlId:   c.ID,
			Description: "The platform service is assessed against " + c.ID + " by the provider's continuous monitoring.",
		})
	}

	component := oscal.DefinedComponent{
		UUID:        g.objectUUID("cdef-component", InheritedComponentSlug),
		Type:        "service",
		Title:       InheritedComponentName,
		Description: "Inherited platform service. Leveraged by every boundary in the fixture federation.",
		Purpose:     "Issues and renews the certificates the boundaries' public endpoints present.",
		// Props from a namespace Horizon does not own, preserved exactly as
		// issued — CamelCase included — and one with no `ns` at all, which
		// OSCAL reads as the default NIST namespace. The namespace schema is
		// a selective validator and must be seen not to fire on either.
		Props: &[]oscal.Property{
			{Name: "EvaluatedServices", Ns: NamespaceAWS, Value: "acm"},
			{Name: "SeverityLabel", Ns: NamespaceAWS, Value: "MEDIUM"},
			{Name: "implementation-point", Value: "external"},
		},
		ControlImplementations: &[]oscal.ControlImplementationSet{{
			UUID:                    g.objectUUID("cdef-control-implementation", InheritedComponentSlug),
			Source:                  "#" + g.securityHubCatalogUUID(),
			Description:             "Security Hub controls the platform service is assessed against.",
			ImplementedRequirements: reqs,
		}},
	}

	meta := g.baseMetadata("Inherited AWS platform components")
	meta.Remarks = "Carries `source` on its control-implementation, so a control identifier reaching a boundary through this document resolves to the Security Hub catalog rather than to the boundary's profile."

	return oscal.OscalCompleteSchema{ComponentDefinition: &oscal.ComponentDefinition{
		UUID:       g.documentUUID("component-definition", "aws-platform"),
		Metadata:   meta,
		Components: &[]oscal.DefinedComponent{component},
		BackMatter: &oscal.BackMatter{Resources: &[]oscal.Resource{
			resourceFor(g.securityHubCatalogUUID(), "Fixture subset of AWS Security Hub controls", "./catalog-aws-security-hub-subset.json", sha256Hex(securityHubBytes)),
		}},
	}}
}
