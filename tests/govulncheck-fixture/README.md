# govulncheck detection fixture

`testdata/fixture.go` calls `language.ParseAcceptLanguage` from `golang.org/x/text`, one of the two symbols
[GO-2022-1059](https://pkg.go.dev/vuln/GO-2022-1059) (CVE-2022-32149) names. `.github/workflows/govulncheck.yml`
builds it into a throwaway module pinned to `golang.org/x/text v0.3.7`, scans it with the same gate as the real
module, and asserts the advisory is found at symbol level. A scanner that stops matching then fails a check instead
of passing every real scan with nothing found. It is the same idea as `tests/sca-fixture/` and
`tests/trufflehog-fixture/`.

Nothing here is a dependency of this repository:

- **No `go.mod` is committed.** The workflow creates the module at run time, so Syft, Trivy and Dependabot never
  see `x/text v0.3.7`, and the real SBOM does not absorb it.
- **The source sits under `testdata/`,** which the Go tool skips, so `go build ./...`, `go test ./...` and
  golangci-lint over the real module never compile it.

Calling `language.Parse` instead is **not** enough: the advisory names `MatchStrings` and `ParseAcceptLanguage`
only, and a scan of code that merely imports the package reports it at package level, which the gate does not
count.
