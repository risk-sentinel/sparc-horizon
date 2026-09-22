// Package fixtures generates the deterministic OSCAL federation the rest of
// Horizon is developed and tested against.
//
// The phase exit criterion P0 closes on is "UUIDs identical across two
// independent regenerations", which is measurable only if the fixtures come
// from a generator anyone can re-run. Two properties make that hold:
//
//   - Nothing reads the clock, the environment or a system random source.
//     Every date is an offset from BaseDate and every choice comes from a
//     splitmix64 sequence with a constant seed, consumed in a fixed order.
//   - Every identifier is derived, not invented. Objects the key grammar
//     covers get their UUIDs from internal/keys; objects OSCAL identifies
//     natively — documents, parties, components, inventory items — get theirs
//     from the fixture-local scheme below, which is deliberately not the
//     grammar and says so in its hashed input.
//
// Every UUID in the output is provisional. The federation namespace UUID is
// not registered yet (sparc#1155), so the fixtures are regenerated when it
// lands, and freezing them is not this package's to do.
package fixtures

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/risk-sentinel/sparc-horizon/internal/canonical"
	"github.com/risk-sentinel/sparc-horizon/internal/keys"
)

// BaseDate anchors every date in the fixtures. The generator never reads the
// clock: a fixture that changes between runs is not a fixture, and the
// regeneration-stability check would measure the date rather than the
// grammar.
var BaseDate = time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

// Seed is the constant the deterministic sequence starts from.
const Seed uint64 = 0x5041524332303236 // "SPARC2026"

// localScheme prefixes every fixture-local identifier. The key grammar's
// hashed input always begins with its version ("v1"), so nothing derived here
// can collide with a real key however the field lists change.
const localScheme = "sparc-horizon-fixture"

// Generator builds the fixture federation under one federation namespace.
type Generator struct {
	ns   uuid.UUID
	keys keys.Deriver
	rng  *prng
}

// New returns a generator deriving under ns. Callers pass
// keys.ProvisionalNamespace() until sparc#1155 registers the real one.
func New(ns uuid.UUID) *Generator {
	return &Generator{ns: ns, keys: keys.New(ns), rng: newPRNG(Seed)}
}

// documentUUID identifies a fixture document. Document-level UUIDs change per
// revision, as the OSCAL spec intends — a revision is a different document —
// so DocumentVersion is part of the input here and nowhere else.
func (g *Generator) documentUUID(kind, slug string) string {
	return g.localUUID("document", kind, slug, DocumentVersion)
}

// objectUUID identifies something OSCAL models natively and the key grammar
// says nothing about: a party, a component, an inventory item, a statement.
// It is stable across revisions.
func (g *Generator) objectUUID(parts ...string) string {
	return g.localUUID(append([]string{"object"}, parts...)...)
}

func (g *Generator) localUUID(parts ...string) string {
	input := strings.Join(append([]string{localScheme}, parts...), canonical.Separator)
	return uuid.NewSHA1(g.ns, []byte(input)).String()
}

// days returns a date offset from BaseDate.
func days(n int) time.Time { return BaseDate.AddDate(0, 0, n) }

// sha256Hex is the digest of the bytes a fixture actually emits, so every
// back-matter hash in the tree is a hash of a file that is really there.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// OSCAL values carry "&" and "<" in prose and hrefs; escaping them to
	// & would make the fixtures diff badly against anything that does
	// not.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Tree is the generated fixture tree: repository-relative path to file bytes.
type Tree map[string][]byte

// Paths returns the tree's paths in sorted order, so every consumer walks it
// the same way.
func (t Tree) Paths() []string {
	out := make([]string, 0, len(t))
	for p := range t {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Generate builds the whole fixture tree in memory.
//
// The order is load-bearing: a document that references another by hash is
// built after the document it hashes, so every rlink digest is the digest of
// bytes this same run produced.
func (g *Generator) Generate() (Tree, error) {
	tree := Tree{}
	add := func(path string, doc any) ([]byte, error) {
		b, err := encode(doc)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		tree[path] = b
		return b, nil
	}

	catBytes, err := add(pathNISTCatalog, g.nistCatalog())
	if err != nil {
		return nil, err
	}
	securityHubBytes, err := add(pathSecurityHubCatalog, g.securityHubCatalog())
	if err != nil {
		return nil, err
	}
	profileBytes, err := add(pathProfile, g.profile(catBytes))
	if err != nil {
		return nil, err
	}
	if _, err := add(pathComponentDefinition, g.awsComponentDefinition(securityHubBytes)); err != nil {
		return nil, err
	}

	// The provider's exported responsibilities are read by every consumer, so
	// they are derived once, and the provider's SSP is built first so the
	// consumers can hash the document they leverage.
	exports, err := g.exportedResponsibilities()
	if err != nil {
		return nil, err
	}

	var providerSSPBytes []byte
	for _, b := range providerFirst(Boundaries) {
		sspBytes, err := add(pathSSP(b), g.ssp(b, profileBytes, providerSSPBytes, exports))
		if err != nil {
			return nil, err
		}
		if b.Slug == ProviderBoundarySlug {
			providerSSPBytes = sspBytes
		}

		artifacts := g.evidenceArtifacts(b)
		for i, a := range artifacts {
			written, err := add(a.path, a.doc)
			if err != nil {
				return nil, err
			}
			// The digest the back-matter resource cites is of the bytes just
			// written, not of a string that stands for them.
			artifacts[i].digest = sha256Hex(written)
		}

		assessed, err := g.assess(b, artifacts, sspBytes)
		if err != nil {
			return nil, err
		}
		if _, err := add(pathAssessmentResults(b), assessed.results); err != nil {
			return nil, err
		}
		if _, err := add(pathPOAM(b), assessed.poam); err != nil {
			return nil, err
		}
	}

	for path, doc := range g.sparcAPI() {
		if _, err := add(path, doc); err != nil {
			return nil, err
		}
	}

	tree[pathREADME] = g.readme(tree)
	return tree, nil
}

// providerFirst orders the boundaries so the one that exports inherited
// controls is generated before the boundaries that leverage it. A consumer
// records the hash of the provider's SSP, which has to exist first.
func providerFirst(in []Boundary) []Boundary {
	out := make([]Boundary, 0, len(in))
	for _, b := range in {
		if b.Slug == ProviderBoundarySlug {
			out = append(out, b)
		}
	}
	for _, b := range in {
		if b.Slug != ProviderBoundarySlug {
			out = append(out, b)
		}
	}
	return out
}

// Write generates the tree and writes it under dir, replacing what is there.
func (g *Generator) Write(dir string) error {
	tree, err := g.Generate()
	if err != nil {
		return err
	}
	for _, p := range tree.Paths() {
		full := filepath.Join(dir, filepath.FromSlash(p))
		// Tight modes: git records only the executable bit, so nothing about
		// the committed tree depends on these, and a generator that widens
		// permissions for no reason is one more thing to explain.
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(full, tree[p], 0o600); err != nil {
			return err
		}
	}
	return nil
}
