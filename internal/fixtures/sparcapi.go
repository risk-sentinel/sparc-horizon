package fixtures

import "strconv"

// The relational half of the seam.
//
// SPARC's Delivery API addresses these objects by numeric id and slug, with
// pagination; Horizon joins on OSCAL UUIDs. Fixtures invented independently of
// that seam would let P1's client pass here and fail against the real thing,
// so every row carries both schemes and the join between them is explicit.
//
// The envelope shape is illustrative. It was written from SPARC's API
// documentation rather than from live responses, and wants confirming against
// an instance before anything here is treated as a contract — the same caveat
// #36 records about the casing evidence.

type collection struct {
	Collection string `json:"collection"`
	Endpoint   string `json:"endpoint"`
	Note       string `json:"note"`
	Count      int    `json:"count"`
	Data       any    `json:"data"`
}

type federationPeerRow struct {
	ID             int    `json:"id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	ShortName      string `json:"short_name"`
	OSCALPartyUUID string `json:"oscal_party_uuid"`
}

type organizationRow struct {
	ID             int    `json:"id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	ShortName      string `json:"short_name"`
	FederationSlug string `json:"federation_slug"`
	OSCALPartyUUID string `json:"oscal_party_uuid"`
}

type boundaryMembershipRow struct {
	ID                        int    `json:"id"`
	OrganizationID            int    `json:"organization_id"`
	OrganizationSlug          string `json:"organization_slug"`
	AuthorizationBoundaryID   int    `json:"authorization_boundary_id"`
	AuthorizationBoundarySlug string `json:"authorization_boundary_slug"`
	OSCALParentPartyUUID      string `json:"oscal_parent_party_uuid"`
	OSCALSSPUUID              string `json:"oscal_ssp_uuid"`
}

type authorizationBoundaryRow struct {
	ID               int    `json:"id"`
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	OrganizationID   int    `json:"organization_id"`
	OrganizationSlug string `json:"organization_slug"`
	FIPS199          string `json:"fips_199"`
	NextDecisionDate string `json:"next_decision_date"`
	OSCALSSPUUID     string `json:"oscal_ssp_uuid"`
}

type sspDocumentRow struct {
	ID                      int    `json:"id"`
	Slug                    string `json:"slug"`
	Title                   string `json:"title"`
	AuthorizationBoundaryID int    `json:"authorization_boundary_id"`
	OSCALUUID               string `json:"oscal_uuid"`
	OSCALVersion            string `json:"oscal_version"`
	Export                  string `json:"export_path"`
}

func (g *Generator) sparcAPI() map[string]any {
	peers := []federationPeerRow{{
		ID:             1,
		Slug:           FederationSlug,
		Name:           FederationName,
		ShortName:      FederationShort,
		OSCALPartyUUID: g.federationPartyUUID(),
	}}

	orgs := make([]organizationRow, 0, len(Orgs))
	for _, o := range Orgs {
		orgs = append(orgs, organizationRow{
			ID:             o.ID,
			Slug:           o.Slug,
			Name:           o.Name,
			ShortName:      o.Short,
			FederationSlug: FederationSlug,
			OSCALPartyUUID: g.orgPartyUUID(o),
		})
	}

	boundaries := make([]authorizationBoundaryRow, 0, len(Boundaries))
	memberships := make([]boundaryMembershipRow, 0, len(Boundaries))
	documents := make([]sspDocumentRow, 0, len(Boundaries))
	for i, b := range Boundaries {
		org := b.Org()
		boundaries = append(boundaries, authorizationBoundaryRow{
			ID:               b.ID,
			Slug:             b.Slug,
			Name:             b.Name,
			OrganizationID:   org.ID,
			OrganizationSlug: org.Slug,
			FIPS199:          b.FIPS,
			NextDecisionDate: b.NextDecision,
			OSCALSSPUUID:     g.sspUUID(b),
		})
		memberships = append(memberships, boundaryMembershipRow{
			ID:                        i + 1,
			OrganizationID:            org.ID,
			OrganizationSlug:          org.Slug,
			AuthorizationBoundaryID:   b.ID,
			AuthorizationBoundarySlug: b.Slug,
			OSCALParentPartyUUID:      g.orgPartyUUID(org),
			OSCALSSPUUID:              g.sspUUID(b),
		})
		documents = append(documents, sspDocumentRow{
			ID:                      b.ID,
			Slug:                    b.Slug + "-ssp",
			Title:                   b.Name + " system security plan",
			AuthorizationBoundaryID: b.ID,
			OSCALUUID:               g.sspUUID(b),
			OSCALVersion:            OSCALVersion,
			Export:                  "../" + pathSSP(b),
		})
	}

	return map[string]any{
		pathSPARC("federation_peers"): collection{
			Collection: "federation_peers",
			Endpoint:   "GET /api/v1/federation_peers",
			Note:       "Federation tier. The OSCAL join key is the party UUID, which appears in every SSP beneath it.",
			Count:      len(peers),
			Data:       peers,
		},
		pathSPARC("organizations"): collection{
			Collection: "organizations",
			Endpoint:   "GET /api/v1/organizations",
			Note:       "Organization tier. oscal_party_uuid is what `parent-uuid` on a boundary SSP points at.",
			Count:      len(orgs),
			Data:       orgs,
		},
		pathSPARC("boundary_memberships"): collection{
			Collection: "boundary_memberships",
			Endpoint:   "GET /api/v1/boundary_memberships",
			Note:       "The organization-to-boundary edge, carried relationally here and as a join on party UUIDs in the documents.",
			Count:      len(memberships),
			Data:       memberships,
		},
		pathSPARC("authorization_boundaries"): collection{
			Collection: "authorization_boundaries",
			Endpoint:   "GET /api/v1/authorization_boundaries",
			Note:       "Boundary tier. One SSP each.",
			Count:      len(boundaries),
			Data:       boundaries,
		},
		pathSPARC("ssp_documents"): collection{
			Collection: "ssp_documents",
			Endpoint:   "GET /api/v1/ssp_documents",
			Note:       "GET /api/v1/ssp_documents/:slug/export returns SPARC's own shape, not OSCAL. export_path here points at the OSCAL document, which is what Horizon reads.",
			Count:      len(documents),
			Data:       documents,
		},
	}
}

// systemCount is the number of system-tier nodes across the federation, used
// by the README so the count is measured rather than asserted.
func systemCount() int {
	n := 0
	for _, b := range Boundaries {
		n += len(b.Systems)
	}
	return n
}

func itoa(n int) string { return strconv.Itoa(n) }
