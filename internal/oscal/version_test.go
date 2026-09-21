package oscal

import (
	"strings"
	"testing"
)

func TestTypesForRejectsWhatItCannotDecode(t *testing.T) {
	for _, v := range SupportedVersions {
		ts, err := TypesFor(v)
		if err != nil {
			t.Errorf("TypesFor(%q): %v", v, err)
			continue
		}
		if ts.Version != v || ts.newDocument() == nil {
			t.Errorf("TypesFor(%q) returned an unusable type set", v)
		}
	}

	// An unlisted version is refused rather than decoded with the nearest
	// types. Falling back is how a field goes missing quietly: the decode
	// succeeds, the document is short, and nothing says so.
	for _, v := range []string{"1.0.0", "1.2.3", "1.3.0", "", "latest"} {
		if _, err := TypesFor(v); err == nil {
			t.Errorf("TypesFor(%q) was accepted", v)
		}
	}
}

func TestVersionAndModel(t *testing.T) {
	doc := []byte(`{"catalog":{"uuid":"x","metadata":{"title":"t","oscal-version":"1.2.2","version":"1","last-modified":"2026-09-21T00:00:00Z"}}}`)

	v, err := Version(doc)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "1.2.2" {
		t.Errorf("Version = %q", v)
	}
	m, err := Model(doc)
	if err != nil {
		t.Fatalf("Model: %v", err)
	}
	if m != "catalog" {
		t.Errorf("Model = %q", m)
	}

	for _, bad := range [][]byte{[]byte("not json"), []byte(`{"catalog":{}}`), []byte(`{}`)} {
		if _, err := Version(bad); err == nil {
			t.Errorf("Version accepted %q", bad)
		}
	}
	if _, err := Model([]byte("not json")); err == nil {
		t.Error("Model accepted invalid JSON")
	}
	// go-oscal's own helper answers "nonsense" here, which is right for a
	// generic tool and wrong for a consumer that will act on the answer.
	if _, err := Model([]byte(`{"nonsense":{}}`)); err == nil {
		t.Error("Model accepted a document whose root is not an OSCAL model")
	}
	if _, err := Model([]byte(`{"catalog":{},"profile":{}}`)); err == nil {
		t.Error("Model accepted a document with two model roots")
	}
	for _, known := range KnownModels {
		doc := []byte(`{"` + known + `":{}}`)
		if got, err := Model(doc); err != nil || got != known {
			t.Errorf("Model(%s) = %q, %v", known, got, err)
		}
	}
}

func TestStrictDecodeReportsTheFieldItCannotModel(t *testing.T) {
	ts, err := TypesFor("1.2.2")
	if err != nil {
		t.Fatalf("TypesFor: %v", err)
	}
	doc := []byte(`{"catalog":{"uuid":"x","metadata":{"title":"t","oscal-version":"1.2.2","version":"1","last-modified":"2026-09-21T00:00:00Z"},"invented":true}}`)

	err = StrictDecode(doc, ts)
	if err == nil {
		t.Fatal("an unmodelled field was accepted")
	}
	if !strings.Contains(err.Error(), "invented") {
		t.Errorf("the error does not name the field: %v", err)
	}
	if !strings.Contains(err.Error(), "1.2.2") {
		t.Errorf("the error does not name the version it decoded at: %v", err)
	}
}
