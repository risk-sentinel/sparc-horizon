// Command oscalprobe prints the round-trip fidelity matrix for a directory of
// OSCAL documents: every document against every OSCAL version Horizon decodes.
//
// The same measurement runs as a test, against a recorded baseline. This exists
// for the times a person needs to see it — a `go-oscal` bump, a document from a
// peer at an unexpected version, or writing the result into an issue the way
// #26 did.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/risk-sentinel/sparc-horizon/internal/oscal"
)

func main() {
	dir := flag.String("dir", "internal/oscal/testdata/nist", "directory of OSCAL documents to probe")
	flag.Parse()

	paths, err := filepath.Glob(filepath.Join(*dir, "*.json"))
	if err != nil || len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "oscalprobe: no documents in %s\n", *dir)
		os.Exit(1)
	}
	sort.Strings(paths)

	fmt.Printf("%-42s %-7s %-8s %-24s %s\n", "document", "declares", "decoded", "round trip", "strict decode")
	failures := 0
	for _, p := range paths {
		if filepath.Base(p) == "PROVENANCE.json" || filepath.Base(p) == "measurements.json" {
			continue
		}
		b, err := os.ReadFile(filepath.Clean(p))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failures++
			continue
		}
		declared, err := oscal.Version(b)
		if err != nil {
			fmt.Fprintln(os.Stderr, p, err)
			failures++
			continue
		}
		for _, m := range oscal.Probe(b) {
			mark := " "
			if m.TypeVersion == declared {
				mark = "*"
			}
			strict := "clean"
			if m.StrictDecodeError != "" {
				strict = m.StrictDecodeError
			}
			fmt.Printf("%-42s %-7s %s%-7s other=%d timestamp=%-8d %s\n",
				filepath.Base(p), declared, mark, m.TypeVersion, m.Differences, m.TimestampDifferences, strict)
			if m.Differences > 0 {
				failures++
			}
		}
	}
	fmt.Printf("\n* = the version the document declares. %d lossy combinations.\n", failures)
}
