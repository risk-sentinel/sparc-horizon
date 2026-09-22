package fixtures

import (
	"strings"

	"github.com/risk-sentinel/sparc-horizon/internal/keys"
)

// readme is generated with the tree so it cannot describe a version of the
// fixtures that no longer exists. It counts what is actually there rather than
// repeating what the tables say should be.
func (g *Generator) readme(tree Tree) []byte {
	var b strings.Builder

	oscalDocs, evidenceFiles, sparcFiles := 0, 0, 0
	for _, p := range tree.Paths() {
		switch {
		case strings.HasPrefix(p, "oscal/"):
			oscalDocs++
		case strings.HasPrefix(p, "evidence/"):
			evidenceFiles++
		case strings.HasPrefix(p, "sparc/"):
			sparcFiles++
		}
	}

	w := func(lines ...string) {
		for _, l := range lines {
			b.WriteString(l)
			b.WriteString("\n")
		}
	}

	w(
		"# Fixture federation",
		"",
		"**Generated. Do not edit by hand.** Every file here is produced by",
		"`internal/fixtures` and rewritten wholesale by `go run ./cmd/genfixtures`.",
		"A hand edit is lost on the next run and fails the regeneration check before that.",
		"",
		"## What is here",
		"",
		"| Tier | Count |",
		"|---|---|",
		"| Federation | 1 |",
		"| Organizations | "+itoa(len(Orgs))+" |",
		"| Authorization boundaries | "+itoa(len(Boundaries))+" |",
		"| Systems | "+itoa(systemCount())+" |",
		"",
		"| Directory | Files | What |",
		"|---|---|---|",
		"| `oscal/` | "+itoa(oscalDocs)+" | Catalogs, the profile, the inherited component definition, and one SSP, assessment-results and POA&M per boundary |",
		"| `evidence/` | "+itoa(evidenceFiles)+" | The artifacts the back-matter resources hash. Synthetic content; the digests are of these bytes |",
		"| `sparc/` | "+itoa(sparcFiles)+" | SPARC's relational addressing — numeric ids and slugs — carrying the OSCAL UUID on every row |",
		"",
		"## The identifiers are final",
		"",
		"Every UUID here derives under the **registered** federation namespace",
		"(`sparc#1155`, decided 2026-09-21):",
		"",
		"```",
		"namespace = uuidv5(url-namespace, \""+keys.NamespaceURI+"\")",
		"          = "+g.ns.String(),
		"```",
		"",
		"Derived rather than invented, so any peer recomputes it from the published URI",
		"instead of copying a constant. These fixtures were regenerated when it landed;",
		"the provisional identifiers that preceded them are gone.",
		"",
		"The key grammar itself, its field lists and the vectors all three runtimes assert",
		"against live in `sparc:lib/federation/key-grammar.v1.json` (`sparc#1161`).",
		"Horizon consumes that file rather than publishing its own.",
		"",
		"## Two identifier schemes, deliberately",
		"",
		"| Objects | Scheme |",
		"|---|---|",
		"| Attestations, observations, findings, risks, POA&M items, evidence resources, AO decisions, responsibility halves, projection cells | The **key grammar** in `docs/03-data-model.md`, via `internal/keys`. Normative, ported to Ruby and Python |",
		"| Documents, parties, components, inventory items, statements | A **fixture-local** scheme, `uuidv5(namespace, \"sparc-horizon-fixture\" + fields)`. Not normative, and its hashed input can never collide with a key, which always begins with the grammar version |",
		"",
		"A back-matter resource that names another document in this tree uses **that",
		"document's own UUID** as the resource UUID. `source-uuid` resolves to the",
		"resource a `source` names, and a fresh UUID per citing document would give one",
		"catalog a different qualifier in every SSP — the opposite of what the qualifier",
		"is for.",
		"",
		"## Determinism",
		"",
		"Nothing reads the clock, the environment, or a system random source. Dates are",
		"offsets from `"+BaseDate.Format("2006-01-02")+"`, and every choice comes from a splitmix64",
		"sequence with a constant seed, consumed in a fixed order. Regenerating into two",
		"directories and diffing them must produce no output; `TestRegenerationIsStable`",
		"and `TestCommittedFixturesMatch` assert both.",
		"",
		"## Synthetic, and the limits of that",
		"",
		"The control text, system names, evidence artifacts and assessment verdicts are",
		"invented. The **structure** is not: it is what `docs/03-data-model.md` specifies,",
		"and the SPARC endpoint shapes in `sparc/` were written from SPARC's API",
		"documentation rather than from live responses. Confirm them against an instance",
		"before treating them as a contract.",
	)
	return []byte(b.String())
}
