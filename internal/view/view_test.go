package view

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-3"

	"github.com/risk-sentinel/sparc-horizon/internal/tree"
)

var fixtureHorizon = time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

func load(t *testing.T, dir string, opts Options) *View {
	t.Helper()
	set, err := LoadFS(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	v, err := New(set, opts)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func hasProblem(v *View, substr string) bool {
	for _, p := range v.Problems() {
		if strings.Contains(p.Detail, substr) {
			return true
		}
	}
	return false
}

// The fixtures join through the standard relationships with nothing left over.
func TestFixturesJoinCleanly(t *testing.T) {
	v := load(t, "../../fixtures", Options{})
	if len(v.Problems()) != 0 {
		t.Fatalf("problems: %+v", v.Problems())
	}
	if !strings.Contains(v.Summary(), "7 SSP, 7 assessment plan, 7 assessment results, 7 POA&M") {
		t.Errorf("summary %q", v.Summary())
	}
	if n := len(v.Chains()); n < 100 {
		t.Errorf("%d chains: the satisfied findings are not being read", n)
	}
	if len(v.Personas()) == 0 {
		t.Error("no personas derived from the fixtures' role holders")
	}
}

// NIST's example links by file path to XML siblings that are not loaded, so
// every placement is a reported fallback, never a silent one.
func TestNISTPlacementsAreReported(t *testing.T) {
	v := load(t, "../oscal/testdata/nist", Options{RoleAliases: map[string]tree.Role{"owner": tree.SystemOwner}})
	for _, want := range []string{
		"which is not a loaded SSP",
		"placed in ifa_ssp-example.json by the components its observations name",
		"placed in ifa_ssp-example.json by system-id",
		"implicit-federation",
	} {
		if !hasProblem(v, want) {
			t.Errorf("no problem reported containing %q: %+v", want, v.Problems())
		}
	}
	p := v.Personas()[0]
	at := time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)
	next, ok := v.NextAction(p.PartyUUID, p.NodeUUID, at)
	if !ok || next.ControlID != "ac-6.1" {
		t.Errorf("next action %+v", next)
	}
	c, ok := v.Chain(p.PartyUUID, next.Chain)
	if !ok || c.Finding == "" || c.Risk == "" || c.PoamItem == "" {
		t.Errorf("chain %+v: want finding, risk and POA&M item resolved", c)
	}
	e, ok := v.Export(p.PartyUUID, p.NodeUUID)
	if !ok || len(e.Documents) != 3 {
		t.Fatalf("export %+v", e)
	}
	if b, ok := v.Document(p.PartyUUID, e.Documents[0].Href); !ok || len(b) == 0 {
		t.Error("the SSP an export lists does not resolve")
	}
	if _, ok := v.Document("someone-else", e.Documents[0].Href); ok {
		t.Error("a party with no role read a document")
	}
}

func TestControlIDs(t *testing.T) {
	for in, want := range map[string]string{
		"ac-6.1_obj": "ac-6.1", "ac-2_smt.a": "ac-2", "EC2.2_obj": "EC2.2", "si-4": "si-4",
	} {
		if got := controlOfTarget(oscal.FindingTarget{TargetId: in}); got != want {
			t.Errorf("%s → %s, want %s", in, got, want)
		}
	}
	if normal("AC-2 (1)") != normal("ac-2.1") {
		t.Error("AC-2 (1) and ac-2.1 are one control")
	}
	if familyOf("ac-2.1") != "AC" || familyOf("ACM") != "ACM" {
		t.Error("familyOf")
	}
}

func TestParseHorizon(t *testing.T) {
	def := fixtureHorizon
	if got, _ := ParseHorizon("", def); !got.Equal(def) {
		t.Error("empty horizon must default")
	}
	if got, err := ParseHorizon("2027-01-02", def); err != nil || got.Format(DateLayout) != "2027-01-02" {
		t.Errorf("%v %v", got, err)
	}
	if _, err := ParseHorizon("23d", def); err == nil {
		t.Error("a duration is not the contract's date")
	}
}

// What cannot be read is reported, and what is not OSCAL is only counted.
func TestLoadReportsAndCounts(t *testing.T) {
	ssp, _ := os.ReadFile("../oscal/testdata/nist/ifa_ssp-example.json")
	future := regexp.MustCompile(`"oscal-version":\s*"[0-9.]+"`).ReplaceAllString(string(ssp), `"oscal-version": "9.9.9"`)
	set, err := LoadFS(fstest.MapFS{
		"ssp.json":    {Data: ssp},
		"future.json": {Data: []byte(future)},
		"notes.json":  {Data: []byte(`{"heat": []}`)},
		"broken.json": {Data: []byte(`{"system-security-plan": {"uuid": 7}}`)},
		"readme.md":   {Data: []byte("# not json")},
		"huge.json":   {Data: make([]byte, 1), Mode: 0o644},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Docs) != 1 || set.NotOSCAL != 2 {
		t.Errorf("%d docs, %d not OSCAL: %+v", len(set.Docs), set.NotOSCAL, set.Problems)
	}
	var sawVersion, sawDecode bool
	for _, p := range set.Problems {
		sawVersion = sawVersion || (p.Path == "future.json" && strings.Contains(p.Detail, "9.9.9"))
		sawDecode = sawDecode || p.Path == "broken.json"
	}
	if !sawVersion || !sawDecode {
		t.Errorf("problems %+v", set.Problems)
	}
	if _, err := New(&Set{}, Options{}); err == nil {
		t.Error("a set with no SSP must refuse")
	}
}

// Heat over a node the party cannot see, or a node with no rows, is refused.
func TestRefusals(t *testing.T) {
	v := load(t, "../../fixtures", Options{})
	ps := v.Personas()
	var so, other Persona
	for _, p := range ps {
		if p.NodeType == "boundary" && so.ID == "" {
			so = p
		}
	}
	for _, p := range ps {
		if p.NodeType == "boundary" && !v.visible(so.PartyUUID, p.NodeUUID) {
			other = p
			break
		}
	}
	if _, ok := v.Heat(so.PartyUUID, other.NodeUUID, fixtureHorizon); ok {
		t.Error("heat on another boundary")
	}
	if _, ok := v.Export(so.PartyUUID, other.NodeUUID); ok {
		t.Error("export of another boundary")
	}
	if _, ok := v.NextAction(so.PartyUUID, "00000000-0000-5000-8000-000000000000", fixtureHorizon); ok {
		t.Error("next action on an absent node")
	}
	if _, ok := v.Cell(so.PartyUUID, so.NodeUUID, "ZZ", fixtureHorizon); ok {
		t.Error("cell for a column that is not drawn")
	}
}

// A column rolls every row, not just the last (#117).
func TestColumnRollsEveryRow(t *testing.T) {
	good := fixtureHorizon.AddDate(0, 6, 0)
	pass := func(n int) []assessment {
		as := make([]assessment, n)
		for i := range as {
			as[i] = assessment{family: "SI", nist: true, expires: &good}
		}
		return as
	}
	down := append(pass(1), assessment{family: "SI", nist: true, expires: &good, failing: true})
	for name, rows := range map[string][][]assessment{
		"first": {down, pass(2)}, "last": {pass(2), down}, "between": {pass(2), down, pass(2)},
	} {
		data := make([]rowData, 0, len(rows))
		for _, as := range rows {
			data = append(data, rowData{byFamily: map[string][]assessment{"SI": as}, weight: 1})
		}
		cols, _ := orderColumns(columnAggs(data, fixtureHorizon))
		if len(cols) != 1 {
			t.Errorf("down row %s: columns %v, want SI drawn", name, cols)
		}
	}
}
