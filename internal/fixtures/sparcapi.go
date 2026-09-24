package fixtures

import "strconv"

// The relational half of the seam, as SPARC actually serves it.
//
// SPARC's Delivery API addresses these objects by numeric id and slug, with
// pagination; Horizon joins on OSCAL UUIDs. These files are what a client
// unmarshals, so they carry EXACTLY the fields SPARC's serializers emit and
// nothing else — no annotations, no convenience keys, no join shortcuts.
//
// ---- What #70 corrected, and why it mattered ------------------------------
//
// The first version of this file was written from SPARC's API documentation
// rather than from its implementation, and was wrong in four ways that would
// all have passed Horizon's own tests:
//
//  1. The envelope was `{collection, endpoint, note, count, data}`. SPARC
//     renders `{data, meta}` — `endpoint`, `collection` and `note` were
//     fixture annotations that a client would never receive, and `count` is
//     not where the real count lives.
//  2. There was NO PAGINATION. Every index goes through `paginate` in
//     api/v1/base_controller.rb, default 25 (organizations passes 50),
//     overridable with ?items= / ?per_page= up to MAX_PAGINATION_LIMIT = 200.
//     A client written against the old fixtures silently truncates any
//     federation larger than one page and its tests still pass.
//  3. `boundary_memberships` is `authorization_boundary_memberships`.
//  4. Rows carried an invented `oscal_party_uuid`, annotated as "what
//     `parent-uuid` on a boundary SSP points at". THAT FIELD DOES NOT EXIST.
//     Organization#uuid is gen_random_uuid() — the model calls it "the stable
//     audit identifier" — and the only party_uuid in SPARC's schema is on
//     ssp_leveraged_authorizations.
//
// The fourth is the one that matters. The relational endpoints are DISCOVERY
// AND ADDRESSING: which documents exist, and which boundary each belongs to.
// The tree join happens on OSCAL party UUIDs read from the documents, which is
// what docs/03-data-model.md specifies. Putting a party UUID on these rows
// encoded the opposite architecture, and a generator that emits a field the
// server does not send is not a fixture, it is a wish.
//
// Confirmed against risk-sentinel/sparc origin/main (bb82c75f) by reading the
// controllers and serializers — stronger than the documentation these were
// first written from, but STILL NOT A LIVE RESPONSE. Confirm against an
// instance before treating any of it as a contract.

// The page envelope every SPARC index returns.
type page struct {
	Data any      `json:"data"`
	Meta pageMeta `json:"meta"`
}

// paginate() in api/v1/base_controller.rb builds exactly these four keys.
type pageMeta struct {
	Page  int `json:"page"`
	Pages int `json:"pages"`
	Count int `json:"count"`
	Items int `json:"items"`
}

// Rows below mirror each controller's `serialize*` method for its INDEX
// action. Fields a controller only adds when `detailed: true` — that is, on
// show — are deliberately absent: an index response does not carry them, and a
// client that reads one from a list is reading a field it will not get.

// serialize_peer in federation_peers_controller.rb.
// No OSCAL identity here: a peer is addressed by name and base_url.
type federationPeerRow struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	BaseURL          string `json:"base_url"`
	Enabled          bool   `json:"enabled"`
	LastSyncedAt     any    `json:"last_synced_at"`
	LastSyncStatus   any    `json:"last_sync_status"`
	ServiceTokenSet  bool   `json:"service_token_set"`
	SigningSecretSet bool   `json:"signing_secret_set"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// serialize in organizations_controller.rb.
// `uuid` is Postgres gen_random_uuid(), an audit identifier — NOT an OSCAL
// party UUID. Joining on it would join on nothing.
type organizationRow struct {
	ID            int    `json:"id"`
	UUID          string `json:"uuid"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Active        bool   `json:"active"`
	MemberCount   int    `json:"member_count"`
	BoundaryCount int    `json:"boundary_count"`
}

// serialize_membership in authorization_boundary_memberships_controller.rb.
//
// `role` and `role_label` are SPARC's OWN membership roles, keyed by user
// email. Horizon MUST NOT read authorization from them: roles come from
// `responsible-parties` in the documents, never from an admin screen
// (docs/07-security.md). They are here because the endpoint sends them, and
// leaving them out would hide the trap rather than document it.
type boundaryMembershipRow struct {
	ID                      int    `json:"id"`
	UserName                string `json:"user_name"`
	UserEmail               string `json:"user_email"`
	UserID                  int    `json:"user_id"`
	Role                    string `json:"role"`
	RoleLabel               string `json:"role_label"`
	AuthorizationBoundaryID int    `json:"authorization_boundary_id"`
	CreatedAt               string `json:"created_at"`
	UpdatedAt               string `json:"updated_at"`
}

// serialize_boundary in authorization_boundaries_controller.rb.
// No UUID of any kind, and no FIPS 199 level: the index carries neither.
// Horizon reads the categorisation from the SSP, not from here.
type authorizationBoundaryRow struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// serialize_document plus append_oscal_fields in ssp_documents_controller.rb.
//
// `uuid` here IS the OSCAL document UUID, and it is the only OSCAL identifier
// anywhere in the relational half — the hinge between addressing and the
// documents Horizon joins on.
type sspDocumentRow struct {
	ID                      int    `json:"id"`
	Slug                    string `json:"slug"`
	UUID                    string `json:"uuid"`
	Name                    string `json:"name"`
	Status                  string `json:"status"`
	LifecycleStatus         string `json:"lifecycle_status"`
	ContentComplete         bool   `json:"content_complete"`
	ContentCompletenessGaps []any  `json:"content_completeness_gaps"`
	FileType                string `json:"file_type"`
	CreationMethod          string `json:"creation_method"`
	AuthorizationBoundaryID int    `json:"authorization_boundary_id"`
	CreatedAt               string `json:"created_at"`
	UpdatedAt               string `json:"updated_at"`
	Published               bool   `json:"published"`
	BackMatterResourceCount int    `json:"back_matter_resources_count"`
}

// sparcAPIPageSize is the page size these fixtures were captured at.
//
// SPARC's real defaults are 25, and 50 for organizations, which no collection
// in this federation reaches — so fixtures captured at the default would all
// be single-page, and a client that ignored `meta` would pass every one of
// them. Capturing at 5 represents `?items=5`, which the API accepts, and makes
// authorization_boundaries, authorization_boundary_memberships and
// ssp_documents span two pages each.
//
// This is the same reasoning as the planted scanner fixtures: a check that
// cannot fail is not a check.
const sparcAPIPageSize = 5

// paginateRows splits rows into the page files SPARC would serve for them,
// keyed by the path each page lands at. Page 1 keeps the collection's plain
// name so the common case reads naturally; later pages carry `.page2` and on,
// matching `?page=`.
func paginateRows[T any](collection string, rows []T, perPage int) map[string]any {
	total := len(rows)
	pages := (total + perPage - 1) / perPage
	if pages == 0 {
		pages = 1
	}

	out := make(map[string]any, pages)
	for p := 1; p <= pages; p++ {
		lo := (p - 1) * perPage
		hi := min(lo+perPage, total)

		path := pathSPARC(collection)
		if p > 1 {
			path = pathSPARC(collection + ".page" + strconv.Itoa(p))
		}
		out[path] = page{
			Data: rows[lo:hi],
			Meta: pageMeta{Page: p, Pages: pages, Count: total, Items: perPage},
		}
	}
	return out
}

// stamp formats a timestamp the way Rails' .iso8601 does, from the generator's
// fixed BaseDate so regeneration is byte-stable.
func stamp(dayOffset int) string { return days(dayOffset).Format("2006-01-02T15:04:05Z") }

func (g *Generator) sparcAPI() map[string]any {
	peers := []federationPeerRow{{
		ID:      1,
		Name:    FederationName,
		BaseURL: "https://sparc." + FederationSlug + ".example",
		Enabled: true,
		// A peer that has never synced reports null for both, which is the
		// state a client is most likely to mishandle.
		LastSyncedAt:     nil,
		LastSyncStatus:   nil,
		ServiceTokenSet:  true,
		SigningSecretSet: true,
		CreatedAt:        stamp(0),
		UpdatedAt:        stamp(0),
	}}

	// boundary_count is what SPARC reports; count it rather than hardcode it,
	// so adding a boundary to the table keeps the fixture honest.
	boundariesPerOrg := map[string]int{}
	for _, b := range Boundaries {
		boundariesPerOrg[b.OrgSlug]++
	}

	orgs := make([]organizationRow, 0, len(Orgs))
	for _, o := range Orgs {
		orgs = append(orgs, organizationRow{
			ID: o.ID,
			// gen_random_uuid() upstream, so Horizon cannot derive it. The
			// fixture-local scheme gives a stable stand-in; it is NOT the
			// OSCAL party UUID and nothing may join on it.
			UUID:          g.localUUID("sparc-record", "organization", o.Slug),
			Slug:          o.Slug,
			Name:          o.Name,
			Active:        true,
			MemberCount:   3,
			BoundaryCount: boundariesPerOrg[o.Slug],
		})
	}

	boundaries := make([]authorizationBoundaryRow, 0, len(Boundaries))
	memberships := make([]boundaryMembershipRow, 0, len(Boundaries))
	documents := make([]sspDocumentRow, 0, len(Boundaries))
	for i, b := range Boundaries {
		boundaries = append(boundaries, authorizationBoundaryRow{
			ID:          b.ID,
			Slug:        b.Slug,
			Name:        b.Name,
			Description: b.Description,
			Status:      "active",
			CreatedAt:   stamp(0),
			UpdatedAt:   stamp(i),
		})
		memberships = append(memberships, boundaryMembershipRow{
			ID:        i + 1,
			UserName:  b.Short + " System Owner",
			UserEmail: b.Slug + "-so@" + FederationSlug + ".example",
			UserID:    100 + i,
			// SPARC's own membership role. Horizon does not read it.
			Role:                    "system_owner",
			RoleLabel:               "System Owner",
			AuthorizationBoundaryID: b.ID,
			CreatedAt:               stamp(0),
			UpdatedAt:               stamp(i),
		})
		documents = append(documents, sspDocumentRow{
			ID:                      b.ID,
			Slug:                    b.Slug + "-ssp",
			UUID:                    g.sspUUID(b),
			Name:                    b.Name + " system security plan",
			Status:                  "completed",
			LifecycleStatus:         "published",
			ContentComplete:         true,
			ContentCompletenessGaps: []any{},
			FileType:                "json",
			CreationMethod:          "import",
			AuthorizationBoundaryID: b.ID,
			CreatedAt:               stamp(0),
			UpdatedAt:               stamp(i),
			Published:               true,
			BackMatterResourceCount: 2,
		})
	}

	out := map[string]any{}
	for path, doc := range paginateRows("federation_peers", peers, sparcAPIPageSize) {
		out[path] = doc
	}
	for path, doc := range paginateRows("organizations", orgs, sparcAPIPageSize) {
		out[path] = doc
	}
	for path, doc := range paginateRows("authorization_boundaries", boundaries, sparcAPIPageSize) {
		out[path] = doc
	}
	// Renamed from `boundary_memberships`, which is not a SPARC route.
	for path, doc := range paginateRows("authorization_boundary_memberships", memberships, sparcAPIPageSize) {
		out[path] = doc
	}
	for path, doc := range paginateRows("ssp_documents", documents, sparcAPIPageSize) {
		out[path] = doc
	}
	return out
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
