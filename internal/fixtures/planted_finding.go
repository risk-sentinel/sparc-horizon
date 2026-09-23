package fixtures

// PlantedDuplicateLiteral exists only to make SonarCloud report a go:S1192
// finding on a pull request, so that sonar-pr-findings.yml can be proved to
// render a non-empty result set and upload the OHDF. The branch it lives on is
// scratch and is never merged.
//
// It is in a production file on purpose. The first attempt put it in a
// _test.go file to protect the coverage floor, and nothing fired:
// sonar-project.properties sets sonar.test.inclusions=**/*_test.go, and
// SonarCloud applies a reduced rule set to test sources.
func PlantedDuplicateLiteral() []string {
	return []string{
		"planted-duplicate-literal-for-s1192",
		"planted-duplicate-literal-for-s1192",
		"planted-duplicate-literal-for-s1192",
		"planted-duplicate-literal-for-s1192",
	}
}
