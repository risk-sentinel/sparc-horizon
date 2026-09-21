package oscal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// update rewrites the recorded measurements. The probe measures something this
// repository does not control, so the baseline is regenerated deliberately and
// reviewed as a diff, never adjusted to make a test pass.
var update = flag.Bool("update", false, "rewrite internal/oscal/testdata/measurements.json")

const (
	corpusDir       = "testdata/nist"
	provenanceFile  = "testdata/nist/PROVENANCE.json"
	measurementFile = "testdata/measurements.json"
	fixturesDir     = "../../fixtures/oscal"
)

type provenance struct {
	About    []string `json:"about"`
	Upstream struct {
		Repository string `json:"repository"`
		Commit     string `json:"commit"`
		Retrieved  string `json:"retrieved"`
		Licence    string `json:"licence"`
	} `json:"upstream"`
	Documents []struct {
		File         string `json:"file"`
		Model        string `json:"model"`
		OSCALVersion string `json:"oscal-version"`
		SHA256       string `json:"sha256"`
		Bytes        int    `json:"bytes"`
		UpstreamPath string `json:"upstream-path"`
	} `json:"documents"`
}

func loadProvenance(t *testing.T) provenance {
	t.Helper()

	b, err := os.ReadFile(provenanceFile)
	if err != nil {
		t.Fatalf("reading provenance: %v", err)
	}
	var p provenance
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("provenance: %v", err)
	}
	return p
}

func readCorpusFile(t *testing.T, name string) []byte {
	t.Helper()

	b, err := fs.ReadFile(os.DirFS(corpusDir), name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return b
}

// A vendored third-party document with no provenance is indistinguishable from
// one somebody edited to make a probe pass.
func TestVendoredDocumentsMatchTheirProvenance(t *testing.T) {
	p := loadProvenance(t)

	if p.Upstream.Commit == "" || p.Upstream.Repository == "" || p.Upstream.Retrieved == "" {
		t.Error("the manifest does not say where the corpus came from or when")
	}

	listed := map[string]bool{}
	for _, d := range p.Documents {
		listed[d.File] = true

		b := readCorpusFile(t, d.File)
		sum := sha256.Sum256(b)
		if got := hex.EncodeToString(sum[:]); got != d.SHA256 {
			t.Errorf("%s: digest %s, manifest says %s", d.File, got, d.SHA256)
		}
		if len(b) != d.Bytes {
			t.Errorf("%s: %d bytes, manifest says %d", d.File, len(b), d.Bytes)
		}

		version, err := Version(b)
		if err != nil {
			t.Errorf("%s: %v", d.File, err)
			continue
		}
		if version != d.OSCALVersion {
			t.Errorf("%s: declares %s, manifest says %s", d.File, version, d.OSCALVersion)
		}
		model, err := Model(b)
		if err != nil {
			t.Errorf("%s: %v", d.File, err)
			continue
		}
		if model != d.Model {
			t.Errorf("%s: is a %s, manifest says %s", d.File, model, d.Model)
		}
	}

	// A file in the directory that the manifest does not list has no
	// provenance at all, which is the case this test exists to catch.
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}
	onDisk := 0
	for _, e := range entries {
		if e.Name() == filepath.Base(provenanceFile) {
			continue
		}
		onDisk++
		if !listed[e.Name()] {
			t.Errorf("%s is in the corpus but not in the manifest", e.Name())
		}
	}
	if onDisk != len(p.Documents) {
		t.Errorf("%d documents on disk, %d in the manifest", onDisk, len(p.Documents))
	}
}

// The corpus has to cover what #26 could not: SSP, profile and POA&M were the
// three models the original spike left unexercised.
func TestCorpusCoversEveryModelHorizonReads(t *testing.T) {
	p := loadProvenance(t)

	models := map[string]bool{}
	for _, d := range p.Documents {
		models[d.Model] = true
	}
	for _, want := range []string{
		"system-security-plan", "profile", "plan-of-action-and-milestones",
		"assessment-results", "catalog", "component-definition", "assessment-plan",
	} {
		if !models[want] {
			t.Errorf("no %s in the corpus", want)
		}
	}
	if len(p.Documents) < 8 {
		t.Errorf("%d documents in the corpus; a probe over a thin corpus proves little", len(p.Documents))
	}
}

// recorded is the committed baseline: every document against every version
// Horizon decodes.
type recorded struct {
	About   []string                 `json:"about"`
	GoOscal string                   `json:"go-oscal"`
	Results map[string][]Measurement `json:"results"`
}

func measureCorpus(t *testing.T) map[string][]Measurement {
	t.Helper()

	p := loadProvenance(t)
	out := map[string][]Measurement{}
	for _, d := range p.Documents {
		out[d.File] = Probe(readCorpusFile(t, d.File))
	}
	return out
}

// The probe measures something this repository does not control, so it is
// recorded rather than asserted. A `go-oscal` bump that fixes a gap, or opens
// one, fails here and has to be looked at — which is the opposite of a test
// written to a guess, which either fails on a true finding or hides one.
func TestProbeMatchesRecordedMeasurements(t *testing.T) {
	measured := measureCorpus(t)

	if *update {
		rec := recorded{
			About: []string{
				"Round-trip fidelity of go-oscal against published OSCAL content (#26, #49).",
				"Each document is decoded with every version's types, re-marshalled, and compared path by path.",
				"differences = fields lost, gained or changed. timestamp-differences = the same instant in different bytes.",
				"strict-decode-error = a field the types do not model at all, found with DisallowUnknownFields.",
				"Regenerate with: go test ./internal/oscal/ -update",
			},
			GoOscal: goOscalVersion(t),
			Results: measured,
		}
		b, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			t.Fatalf("encoding measurements: %v", err)
		}
		if err := os.WriteFile(measurementFile, append(b, '\n'), 0o600); err != nil {
			t.Fatalf("writing measurements: %v", err)
		}
		t.Log("measurements rewritten; review the diff")
		return
	}

	b, err := os.ReadFile(measurementFile)
	if err != nil {
		t.Fatalf("reading measurements: %v — run `go test ./internal/oscal/ -update`", err)
	}
	var rec recorded
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("measurements: %v", err)
	}

	if len(rec.Results) != len(measured) {
		t.Errorf("recorded %d documents, measured %d", len(rec.Results), len(measured))
	}
	combinations := 0
	for file, want := range rec.Results {
		got, ok := measured[file]
		if !ok {
			t.Errorf("%s: recorded but no longer in the corpus", file)
			continue
		}
		if len(got) != len(want) {
			t.Errorf("%s: %d measurements, recorded %d", file, len(got), len(want))
			continue
		}
		for i := range want {
			combinations++
			if diff := describe(got[i], want[i]); diff != "" {
				t.Errorf("%s at %s: %s — run `go test ./internal/oscal/ -update` and review the diff",
					file, want[i].TypeVersion, diff)
			}
		}
	}
	// A comparison loop over an empty baseline passes while measuring nothing.
	if wantCombinations := len(measured) * len(SupportedVersions); combinations != wantCombinations {
		t.Errorf("compared %d document/version combinations, want %d", combinations, wantCombinations)
	}
}

func describe(got, want Measurement) string {
	switch {
	case got.TypeVersion != want.TypeVersion:
		return "version " + got.TypeVersion + ", recorded " + want.TypeVersion
	case got.Differences != want.Differences:
		return "lost " + itoa(got.Differences) + " fields, recorded " + itoa(want.Differences)
	case got.TimestampDifferences != want.TimestampDifferences:
		return "normalised " + itoa(got.TimestampDifferences) + " timestamps, recorded " + itoa(want.TimestampDifferences)
	case got.StrictDecodeError != want.StrictDecodeError:
		return "strict decode said " + quote(got.StrictDecodeError) + ", recorded " + quote(want.StrictDecodeError)
	case strings.Join(got.Paths, ",") != strings.Join(want.Paths, ","):
		return "lost " + quote(strings.Join(got.Paths, ",")) + ", recorded " + quote(strings.Join(want.Paths, ","))
	}
	return ""
}

// The finding, stated as an assertion rather than absorbed into the baseline.
//
// go-oscal shares one LocalDefinitions struct between an assessment plan and a
// result, and OSCAL's result-local-definitions carries `tasks` while the plan's
// does not. The field is therefore never modelled, at any version, and a
// schema-valid assessment-results document loses it on the way through. #26
// reported "no unknown fields in any of the five" documents it probed; none of
// them used this assembly.
func TestTheOnlyLossIsResultLocalDefinitionsTasks(t *testing.T) {
	const knownPath = "/assessment-results/results/0/local-definitions/tasks"

	lossy := map[string][]string{}
	for file, measurements := range measureCorpus(t) {
		for _, m := range measurements {
			for _, p := range m.Paths {
				lossy[file] = append(lossy[file], m.TypeVersion+" "+p)
			}
		}
	}

	for file, losses := range lossy {
		for _, loss := range losses {
			if !strings.HasSuffix(loss, knownPath) {
				t.Errorf("%s: a second gap in the type layer, at %s. This is a new finding, not a baseline to update", file, loss)
			}
		}
	}

	// And the known one is still there: if it silently stops being reported,
	// either go-oscal fixed it — worth knowing — or the probe stopped probing.
	if len(lossy) != 1 {
		t.Errorf("%d documents lose fields, want exactly the one known case: %v", len(lossy), lossy)
	}
	if got := len(lossy["ifa_assessment-results.json"]); got != len(SupportedVersions) {
		t.Errorf("the known gap appears at %d of %d versions; it is version-independent", got, len(SupportedVersions))
	}
}

// The fixtures are probed too, and this is deliberately the weaker claim: they
// were written by go-oscal (#36), so it says the emitter and the parser agree,
// not that the type layer is faithful to OSCAL. Running it anyway catches a
// generator that emits something it cannot read back.
func TestFixtureFederationIsSelfConsistent(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(fixturesDir, "*.json"))
	if err != nil {
		t.Fatalf("globbing fixtures: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no fixture documents found — run `go run ./cmd/genfixtures`")
	}

	probed := 0
	for _, p := range paths {
		b, err := os.ReadFile(filepath.Clean(p))
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		version, err := Version(b)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		ts, err := TypesFor(version)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		diffs, err := RoundTrip(b, ts)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		for _, d := range Only(diffs, DiffOther) {
			t.Errorf("%s: %s", filepath.Base(p), d)
		}
		if err := StrictDecode(b, ts); err != nil {
			t.Errorf("%s: %v", filepath.Base(p), err)
		}
		probed++
	}
	if probed != len(paths) {
		t.Errorf("probed %d of %d fixture documents", probed, len(paths))
	}
}

func goOscalVersion(t *testing.T) string {
	t.Helper()

	b, err := os.ReadFile("../../go.mod")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "defenseunicorns/go-oscal") {
			fields := strings.Fields(line)
			return fields[len(fields)-1]
		}
	}
	return "unknown"
}

func itoa(n int) string     { return strconv.Itoa(n) }
func quote(s string) string { return strconv.Quote(s) }
