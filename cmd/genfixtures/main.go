// Command genfixtures writes the deterministic fixture federation.
//
// It takes no seed and no date: both are constants in internal/fixtures,
// because the phase exit criterion is that two independent regenerations agree
// and a flag that changes the output is a flag that can be set differently
// twice.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/risk-sentinel/sparc-horizon/internal/fixtures"
	"github.com/risk-sentinel/sparc-horizon/internal/keys"
)

func main() {
	out := flag.String("out", "fixtures", "directory to write the fixture federation into")
	flag.Parse()

	// The federation namespace is still provisional; sparc#1155 has not
	// registered the real one.
	g := fixtures.New(keys.ProvisionalNamespace())
	if err := g.Write(*out); err != nil {
		fmt.Fprintln(os.Stderr, "genfixtures:", err)
		os.Exit(1)
	}
}
