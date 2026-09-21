package oscal

import (
	"strings"
	"testing"
)

func mustTypes(t *testing.T, version string) TypeSet {
	t.Helper()

	ts, err := TypesFor(version)
	if err != nil {
		t.Fatalf("TypesFor(%q): %v", version, err)
	}
	return ts
}

// The difference #26 measured, and the reason signatures are verified over
// received bytes: the same instant, different bytes, because JCS canonicalises
// member order and number formatting but not string values.
func TestTimestampNormalisationIsClassifiedNotHidden(t *testing.T) {
	ts := mustTypes(t, "1.2.2")
	doc := []byte(`{"catalog":{"uuid":"c","metadata":{"title":"t","version":"1","oscal-version":"1.2.2","last-modified":"2026-03-31T19:45:48.195797+00:00"}}}`)

	diffs, err := RoundTrip(doc, ts)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if got := len(Only(diffs, DiffOther)); got != 0 {
		t.Errorf("%d real differences in a document that only changed timezone spelling: %v", got, diffs)
	}
	stamps := Only(diffs, DiffTimestamp)
	if len(stamps) != 1 {
		t.Fatalf("%d timestamp differences, want 1: %v", len(stamps), diffs)
	}
	if !strings.HasSuffix(stamps[0].Path, "/last-modified") {
		t.Errorf("classified the wrong path: %s", stamps[0].Path)
	}
	// It is reported, not swallowed. A probe that tolerated it silently would
	// stop being able to say why the signing rule exists.
	if !strings.Contains(stamps[0].String(), "+00:00") || !strings.Contains(stamps[0].String(), "Z") {
		t.Errorf("the difference does not show both spellings: %s", stamps[0])
	}
}

func TestRoundTripRejectsWhatItCannotParse(t *testing.T) {
	ts := mustTypes(t, "1.2.2")
	if _, err := RoundTrip([]byte("{"), ts); err == nil {
		t.Error("truncated JSON was round-tripped")
	}
	if _, err := decodeGeneric([]byte("{")); err == nil {
		t.Error("truncated JSON decoded for comparison")
	}
}

// The comparison walks structure, not text, so member order and equivalent
// number spellings are not reported as loss — but a missing field, an extra
// one, a changed value and a changed shape all are.
func TestCompareFindsStructuralDifferences(t *testing.T) {
	cases := []struct {
		name          string
		source, after string
		wantPaths     []string
	}{
		{"identical", `{"a":1,"b":[1,2]}`, `{"b":[1,2],"a":1}`, nil},
		{"number spelling", `{"a":1}`, `{"a":1}`, nil},
		{"field lost", `{"a":1,"b":2}`, `{"a":1}`, []string{"/b"}},
		{"field gained", `{"a":1}`, `{"a":1,"b":2}`, []string{"/b"}},
		{"value changed", `{"a":1}`, `{"a":2}`, []string{"/a"}},
		{"shape changed", `{"a":{"x":1}}`, `{"a":[1]}`, []string{"/a"}},
		{"array shortened", `{"a":[1,2,3]}`, `{"a":[1,2]}`, []string{"/a"}},
		{"array element changed", `{"a":[1,2]}`, `{"a":[1,3]}`, []string{"/a/1"}},
		{"nested", `{"a":{"b":{"c":"x"}}}`, `{"a":{"b":{"c":"y"}}}`, []string{"/a/b/c"}},
	}

	for _, c := range cases {
		source, err := decodeGeneric([]byte(c.source))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		after, err := decodeGeneric([]byte(c.after))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}

		var diffs []Difference
		compare("", source, after, &diffs)
		if len(diffs) != len(c.wantPaths) {
			t.Errorf("%s: %d differences, want %d: %v", c.name, len(diffs), len(c.wantPaths), diffs)
			continue
		}
		for i, want := range c.wantPaths {
			if diffs[i].Path != want {
				t.Errorf("%s: difference %d at %q, want %q", c.name, i, diffs[i].Path, want)
			}
			if diffs[i].Kind != DiffOther {
				t.Errorf("%s: %q classified as %s", c.name, diffs[i].Path, diffs[i].Kind)
			}
		}
	}
}

func TestSameInstantOnlyClassifiesRealTimestamps(t *testing.T) {
	if !sameInstant("2026-03-31T19:45:48.195797+00:00", "2026-03-31T19:45:48.195797Z") {
		t.Error("two spellings of one instant were not recognised")
	}
	for _, c := range [][2]any{
		{"2026-03-31T19:45:48Z", "2026-03-31T19:45:49Z"}, // a second apart is a real change
		{"2026-03-31", "2026-04-01"},                     // dates are not timestamps
		{"hello", "world"},
		{"2026-03-31T19:45:48Z", nil},
		{1, 2},
	} {
		if sameInstant(c[0], c[1]) {
			t.Errorf("%v and %v were treated as one instant", c[0], c[1])
		}
	}
}

func TestRenderTruncatesAndNamesAbsence(t *testing.T) {
	if got := render(nil); got != "<absent>" {
		t.Errorf("render(nil) = %q", got)
	}
	if got := render("plain"); got != "plain" {
		t.Errorf("render(string) = %q", got)
	}
	long := make([]any, 200)
	for i := range long {
		long[i] = "padding"
	}
	if got := render(long); !strings.HasSuffix(got, "…") || len(got) > 200 {
		t.Errorf("a large subtree was not truncated: %d chars", len(got))
	}
}

func TestOnlyFiltersByKind(t *testing.T) {
	ds := []Difference{
		{Path: "/a", Kind: DiffOther},
		{Path: "/b", Kind: DiffTimestamp},
		{Path: "/c", Kind: DiffOther},
	}
	if got := len(Only(ds, DiffOther)); got != 2 {
		t.Errorf("Only(DiffOther) returned %d", got)
	}
	if got := len(Only(ds, DiffTimestamp)); got != 1 {
		t.Errorf("Only(DiffTimestamp) returned %d", got)
	}
	if got := Only(nil, DiffOther); got != nil {
		t.Errorf("Only(nil) = %v", got)
	}
}

func TestProbeCoversEveryVersion(t *testing.T) {
	doc := []byte(`{"catalog":{"uuid":"c","metadata":{"title":"t","version":"1","oscal-version":"1.2.2","last-modified":"2026-09-21T00:00:00Z"}}}`)

	ms := Probe(doc)
	if len(ms) != len(SupportedVersions) {
		t.Fatalf("Probe returned %d measurements, want %d", len(ms), len(SupportedVersions))
	}
	for i, m := range ms {
		if m.TypeVersion != SupportedVersions[i] {
			t.Errorf("measurement %d is for %s, want %s", i, m.TypeVersion, SupportedVersions[i])
		}
		if m.Differences != 0 || m.StrictDecodeError != "" {
			t.Errorf("%s: a minimal catalog did not survive: %+v", m.TypeVersion, m)
		}
	}
}
