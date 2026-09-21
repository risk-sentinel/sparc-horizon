package fixtures

// The shape of the fixture federation: four organizations, seven
// authorization boundaries and twenty systems, from #36.
//
// It is a table rather than generated names because a fixture nobody can read
// is a fixture nobody checks. The variation that has to look real — which
// controls pass, when observations expire, which risks block an ATO — is
// derived deterministically in assess.go instead.

// Namespaces. Horizon owns the first and only validates the first; the others
// are preserved exactly as issued, CamelCase included.
const (
	NamespaceSPARC = "https://risk-sentinel.org/ns/sparc"
	NamespaceAWS   = "http://aws.amazon.com/ns/oscal"
)

// OSCALVersion is the version every fixture document declares.
const OSCALVersion = "1.2.2"

// Period is the assessment period every keyed object in the fixtures falls in.
const Period = "2026-Q3"

// DocumentVersion is the fixture revision. Document-level UUIDs change per
// revision in OSCAL; here they are derived from this string, so a bump
// regenerates them and nothing else.
const DocumentVersion = "1.0.0"

// Org is one organization tier node. ID and Slug are SPARC's relational
// addressing; the OSCAL side joins on the party UUID derived from the slug.
type Org struct {
	ID    int
	Slug  string
	Name  string
	Short string
}

// System is one component of a boundary — the system tier.
type System struct {
	Slug    string
	Name    string
	Type    string
	Purpose string
}

// Boundary is one authorization boundary: one SSP, one assessment-results
// document and one POA&M.
type Boundary struct {
	ID           int
	Slug         string
	Name         string
	Short        string
	OrgSlug      string
	FIPS         string
	NextDecision string
	Description  string
	Systems      []System
}

// Control is one control in a catalog, with the family it belongs to.
type Control struct {
	ID     string
	Title  string
	Family string
}

// Federation names the whole tree. It is a party in every SSP beneath it,
// which is what makes the tree a join rather than a hierarchy table.
const (
	FederationSlug  = "cascade"
	FederationName  = "Cascade Federation"
	FederationShort = "CASCADE"
)

// Orgs are the four organization-tier nodes.
var Orgs = []Org{
	{ID: 1, Slug: "ods", Name: "Office of Digital Services", Short: "ODS"},
	{ID: 2, Slug: "bgm", Name: "Bureau of Grants Management", Short: "BGM"},
	{ID: 3, Slug: "ndi", Name: "National Data Institute", Short: "NDI"},
	{ID: 4, Slug: "coa", Name: "Coastal Operations Agency", Short: "COA"},
}

// Boundaries are the seven authorization boundaries, with the twenty systems
// distributed across them.
var Boundaries = []Boundary{
	{
		ID: 1, Slug: "ods-portal", Name: "Public Portal", Short: "PORTAL", OrgSlug: "ods",
		FIPS: "moderate", NextDecision: "2026-10-11",
		Description: "Public-facing service portal and its supporting API tier.",
		Systems: []System{
			{Slug: "portal-web", Name: "Portal Web Tier", Type: "software", Purpose: "Serves the public web interface."},
			{Slug: "portal-api", Name: "Portal API", Type: "software", Purpose: "Application interface behind the web tier."},
			{Slug: "portal-cdn", Name: "Portal Edge Cache", Type: "service", Purpose: "Caches static assets at the edge."},
		},
	},
	{
		ID: 2, Slug: "ods-identity", Name: "Identity Services", Short: "IDENTITY", OrgSlug: "ods",
		FIPS: "high", NextDecision: "2026-11-02",
		Description: "Agency identity provider and directory, leveraged by every other boundary in the federation.",
		Systems: []System{
			{Slug: "idp", Name: "Agency Identity Provider", Type: "software", Purpose: "Authenticates users for every leveraging boundary."},
			{Slug: "idp-dir", Name: "Directory Service", Type: "software", Purpose: "Holds the authoritative user and group records."},
			{Slug: "idp-mfa", Name: "Multi-Factor Service", Type: "service", Purpose: "Issues and verifies second factors."},
		},
	},
	{
		ID: 3, Slug: "bgm-grants", Name: "Grants Platform", Short: "GRANTS", OrgSlug: "bgm",
		FIPS: "high", NextDecision: "2026-10-24",
		Description: "Grant application intake, review and award record.",
		Systems: []System{
			{Slug: "grants-app", Name: "Grants Application", Type: "software", Purpose: "Intake and review workflow."},
			{Slug: "grants-db", Name: "Grants Database", Type: "software", Purpose: "System of record for awards."},
			{Slug: "grants-etl", Name: "Grants ETL", Type: "software", Purpose: "Moves award data to the reporting store."},
		},
	},
	{
		ID: 4, Slug: "bgm-payments", Name: "Payments Gateway", Short: "PAYMENTS", OrgSlug: "bgm",
		FIPS: "high", NextDecision: "2026-12-01",
		Description: "Disbursement gateway and its ledger.",
		Systems: []System{
			{Slug: "pay-gateway", Name: "Payment Gateway", Type: "software", Purpose: "Submits and reconciles disbursements."},
			{Slug: "pay-ledger", Name: "Payment Ledger", Type: "software", Purpose: "Append-only record of disbursements."},
			{Slug: "pay-hsm", Name: "Payment HSM", Type: "hardware", Purpose: "Holds the signing keys for disbursement files."},
		},
	},
	{
		ID: 5, Slug: "ndi-datalake", Name: "Research Data Lake", Short: "DATALAKE", OrgSlug: "ndi",
		FIPS: "moderate", NextDecision: "2027-01-15",
		Description: "Curated research data store and its ingest path.",
		Systems: []System{
			{Slug: "lake-store", Name: "Object Store", Type: "service", Purpose: "Stores curated research datasets."},
			{Slug: "lake-catalog", Name: "Data Catalog", Type: "software", Purpose: "Indexes datasets and their provenance."},
			{Slug: "lake-ingest", Name: "Ingest Pipeline", Type: "software", Purpose: "Validates and loads submitted datasets."},
		},
	},
	{
		ID: 6, Slug: "ndi-analytics", Name: "Analytics Workbench", Short: "WORKBENCH", OrgSlug: "ndi",
		FIPS: "low", NextDecision: "2027-02-20",
		Description: "Analyst notebooks and scheduled jobs over the data lake.",
		Systems: []System{
			{Slug: "wb-notebook", Name: "Notebook Service", Type: "software", Purpose: "Hosts analyst notebooks."},
			{Slug: "wb-scheduler", Name: "Job Scheduler", Type: "software", Purpose: "Runs scheduled analysis jobs."},
			{Slug: "wb-registry", Name: "Model Registry", Type: "software", Purpose: "Versions trained models."},
		},
	},
	{
		ID: 7, Slug: "coa-fieldops", Name: "Field Operations", Short: "FIELDOPS", OrgSlug: "coa",
		FIPS: "moderate", NextDecision: "2026-10-30",
		Description: "Field data capture and its synchronisation service.",
		Systems: []System{
			{Slug: "field-mobile", Name: "Field Mobile Application", Type: "software", Purpose: "Captures observations in the field."},
			{Slug: "field-sync", Name: "Sync Service", Type: "software", Purpose: "Reconciles field captures with the record."},
		},
	},
}

// NISTControls is the SP 800-53 subset the profile resolves. Six families,
// with two enhancements, so the canonical dotted form appears in the fixtures
// rather than only in the vectors.
var NISTControls = []Control{
	{ID: "ac-2", Family: "ac", Title: "Account Management"},
	{ID: "ac-2.1", Family: "ac", Title: "Account Management | Automated System Account Management"},
	{ID: "au-6", Family: "au", Title: "Audit Record Review, Analysis, and Reporting"},
	{ID: "au-6.1", Family: "au", Title: "Audit Record Review, Analysis, and Reporting | Automated Process Integration"},
	{ID: "cp-4", Family: "cp", Title: "Contingency Plan Testing"},
	{ID: "cp-9", Family: "cp", Title: "System Backup"},
	{ID: "ia-2", Family: "ia", Title: "Identification and Authentication (Organizational Users)"},
	{ID: "ia-5", Family: "ia", Title: "Authenticator Management"},
	{ID: "sc-7", Family: "sc", Title: "Boundary Protection"},
	{ID: "sc-28", Family: "sc", Title: "Protection of Information at Rest"},
	{ID: "si-4", Family: "si", Title: "System Monitoring"},
	{ID: "si-7", Family: "si", Title: "Software, Firmware, and Information Integrity"},
}

// NISTFamilies titles the six families, in the order they group the catalog.
var NISTFamilies = []Control{
	{ID: "ac", Title: "Access Control"},
	{ID: "au", Title: "Audit and Accountability"},
	{ID: "cp", Title: "Contingency Planning"},
	{ID: "ia", Title: "Identification and Authentication"},
	{ID: "sc", Title: "System and Communications Protection"},
	{ID: "si", Title: "System and Information Integrity"},
}

// SecurityHubControls is the second authority. Their identifiers are carried
// exactly as AWS issues them: "ACM.1" is not "acm.1", and canonicalising it
// into NIST's form would produce an identifier that validates and names
// nothing. They are in the fixtures so that rule is exercised rather than
// merely written down.
var SecurityHubControls = []Control{
	{ID: "ACM.1", Family: "ACM", Title: "Imported and ACM-issued certificates should be renewed after a specified time period"},
	{ID: "IAM.4", Family: "IAM", Title: "IAM root user access key should not exist"},
	{ID: "S3.5", Family: "S3", Title: "S3 general purpose buckets should require requests to use SSL"},
	{ID: "EC2.2", Family: "EC2", Title: "VPC default security groups should not allow inbound or outbound traffic"},
}

// InheritedPlatformComponent is the AWS service component every boundary
// leverages. It carries the foreign-namespace and no-namespace props that the
// namespace schema must be seen not to fire on.
const (
	InheritedComponentSlug = "aws-acm"
	InheritedComponentName = "AWS Certificate Manager"
)

// ProviderBoundarySlug is the boundary that exports inherited controls; every
// other boundary consumes them. A hybrid control carries both halves, and is
// green only when both have unexpired observations.
const ProviderBoundarySlug = "ods-identity"

// InheritedControls are the controls the provider exports and the consumers
// answer for.
var InheritedControls = []string{"ia-2", "ia-5"}

// Org returns the organization a boundary belongs to, or the zero Org if the
// tables disagree. TestEveryBoundaryNamesAKnownOrganization is what makes the
// zero value unreachable in practice; returning it rather than panicking keeps
// a table typo a failing test instead of a crash in a generator.
func (b Boundary) Org() Org {
	for _, o := range Orgs {
		if o.Slug == b.OrgSlug {
			return o
		}
	}
	return Org{}
}
