// Command fixture exists only to be flagged by govulncheck.
//
// It calls language.ParseAcceptLanguage from golang.org/x/text v0.3.7, one of
// the two symbols GO-2022-1059 (CVE-2022-32149) names. The call makes the
// vulnerable symbol reachable, so a working govulncheck must report it at
// symbol level and exit 3. It is a separate module,
// so the real module graph and `go test ./...` never see it.
package main

import (
	"fmt"
	"os"

	"golang.org/x/text/language"
)

func main() {
	tags, _, err := language.ParseAcceptLanguage(os.Args[len(os.Args)-1])
	fmt.Println(tags, err)
}
