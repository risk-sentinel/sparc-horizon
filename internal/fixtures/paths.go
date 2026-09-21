package fixtures

// Where each fixture lands, relative to the fixtures directory.
const (
	pathNISTCatalog         = "oscal/catalog-nist-800-53-subset.json"
	pathSecurityHubCatalog  = "oscal/catalog-aws-security-hub-subset.json"
	pathProfile             = "oscal/profile-moderate.json"
	pathComponentDefinition = "oscal/component-definition-aws-platform.json"
	pathKeyVectors          = "key-vectors.v1.json"
	pathREADME              = "README.md"
)

func pathSSP(b Boundary) string               { return "oscal/ssp-" + b.Slug + ".json" }
func pathAssessmentResults(b Boundary) string { return "oscal/ar-" + b.Slug + ".json" }
func pathPOAM(b Boundary) string              { return "oscal/poam-" + b.Slug + ".json" }
func pathEvidence(b Boundary, kind string) string {
	return "evidence/" + b.Slug + "-" + kind + "-" + Period + ".json"
}
func pathSPARC(collection string) string { return "sparc/" + collection + ".json" }
