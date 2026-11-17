// Command baseline validates container-baseline.yml and writes the Trivy ignore
// file the release gate uses.
//
//	go run ./cmd/baseline -file container-baseline.yml -trivyignore .trivyignore
//
// It exits non-zero, listing every problem, when the baseline is malformed or
// an accepted finding is past its review date. Both paths are resolved inside
// the working directory (os.Root), so neither can reach outside the checkout.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/risk-sentinel/sparc-horizon/internal/baseline"
)

func main() {
	file := flag.String("file", "container-baseline.yml", "baseline to validate")
	ignore := flag.String("trivyignore", "", "write the Trivy ignore file here (optional)")
	flag.Parse()

	if err := run(*file, *ignore, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(file, ignore string, now time.Time) error {
	root, err := os.OpenRoot(".")
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	f, err := root.Open(file)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	b, err := baseline.Parse(f)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	if errs := b.Validate(now); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "::error::%s: %v\n", file, e)
		}
		return fmt.Errorf("%s: %d problem(s)", file, len(errs))
	}
	fmt.Printf("%s: %d finding(s), %d suppressed (accepted or deferred)\n", file, len(b.Findings), len(b.IgnoreIDs()))

	if ignore == "" {
		return nil
	}
	out, err := root.Create(ignore)
	if err != nil {
		return err
	}
	if err := b.WriteTrivyIgnore(out, file); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
