package fixtures

import "testing"

// Deliberately duplicated literals, planted to make SonarCloud report a
// finding (go:S1192) on a pull request. This exists only to prove that
// sonar-pr-findings.yml renders a non-empty result set and uploads the OHDF.
// The branch it lives on is scratch and is never merged.
func TestPlantedDuplicateLiteral(t *testing.T) {
	a := "planted-duplicate-literal-for-s1192"
	b := "planted-duplicate-literal-for-s1192"
	c := "planted-duplicate-literal-for-s1192"
	d := "planted-duplicate-literal-for-s1192"
	if a != b || b != c || c != d {
		t.Fatal("unreachable")
	}
}
