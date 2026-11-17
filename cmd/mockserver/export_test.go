package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"testing"
)

type exportBody struct {
	Boundary  string `json:"boundary"`
	Documents []struct {
		Model  string `json:"model"`
		Href   string `json:"href"`
		SHA256 string `json:"sha256"`
	} `json:"documents"`
}

// An export is refused the way a node is. Before #117 it checked only that the
// persona existed, so a system owner could list another organization's
// documents by slug.
func TestExportFollowsTheSameRule(t *testing.T) {
	s, h := testServer(t)
	ao, so, iso := s.personas[0], s.personas[1], s.personas[2]

	own := s.byUUID[strings.ToLower(so.NodeUUID)]
	if own == "" {
		t.Fatalf("premise wrong: the SO's node %s is not a boundary with an SSP", so.NodeUUID)
	}
	foreign := s.byUUID[strings.ToLower(iso.NodeUUID)]
	sibling := "ods-identity"
	if !s.authz.Visible(ao.PartyUUID, s.bySlug[sibling]) || s.authz.Visible(so.PartyUUID, s.bySlug[sibling]) {
		t.Fatalf("premise wrong: %s should be visible to the AO and not to the SO", sibling)
	}

	bySlug := get(t, h, so.ID, "/v1/export/oscal/"+own)
	if bySlug.Code != http.StatusOK {
		t.Fatalf("the SO's own boundary by slug returned %d", bySlug.Code)
	}
	for _, ref := range []string{so.NodeUUID, strings.ToUpper(so.NodeUUID)} {
		w := get(t, h, so.ID, "/v1/export/oscal/"+ref)
		if w.Code != http.StatusOK {
			t.Errorf("the SO's own boundary by UUID %s returned %d, want 200", ref, w.Code)
		} else if w.Body.String() != bySlug.Body.String() {
			t.Errorf("by UUID and by slug differ:\n  uuid: %s\n  slug: %s", w.Body.String(), bySlug.Body.String())
		}
	}
	if w := get(t, h, ao.ID, "/v1/export/oscal/"+sibling); w.Code != http.StatusOK {
		t.Errorf("the AO's boundary %s returned %d, want 200", sibling, w.Code)
	}

	missing := get(t, h, so.ID, "/v1/export/oscal/no-such-boundary")
	for _, ref := range []string{foreign, iso.NodeUUID, sibling, "00000000-0000-5000-8000-000000000000"} {
		w := get(t, h, so.ID, "/v1/export/oscal/"+ref)
		if w.Code != http.StatusNotFound || w.Body.String() != missing.Body.String() {
			t.Errorf("%s returned %d %s; want the same 404 as a boundary that does not exist", ref, w.Code, w.Body.String())
		}
	}
}

// Every href an export lists resolves, for the caller who was given it, to the
// document whose sha256 the export stated, and to nobody else.
func TestExportLinksResolveAndVerify(t *testing.T) {
	s, h := testServer(t)
	so, iso := s.personas[1], s.personas[2]

	w := get(t, h, so.ID, "/v1/export/oscal/"+so.NodeUUID)
	var body exportBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(body.Documents) != len(exportModels) {
		t.Fatalf("export lists %d documents, want %d (SSP, AR, POA&M)", len(body.Documents), len(exportModels))
	}
	for i, d := range body.Documents {
		if d.Model != exportModels[i].model {
			t.Errorf("document %d is %s, want %s: the order is part of the response", i, d.Model, exportModels[i].model)
		}
		checkLinkedDocument(t, h, so.ID, iso.ID, d.Href, d.SHA256)
	}

	for _, p := range []string{
		"/oscal/catalog-nist-800-53-subset.json", // not a boundary document
		"/oscal/profile-moderate.json",
		"/oscal/ssp-no-such-boundary.json",
		"/oscal/ssp-ods-portal.json.bak",
		"/oscal/ssp-..%2fapi%2fpersonas.json",
	} {
		if w := get(t, h, so.ID, p); w.Code != http.StatusNotFound {
			t.Errorf("%s returned %d, want 404", p, w.Code)
		}
	}
	if w := get(t, h, "nobody", body.Documents[0].Href); w.Code != http.StatusUnauthorized {
		t.Errorf("an unknown persona fetching a document got %d, want 401", w.Code)
	}
}

// checkLinkedDocument fetches one href as the caller it was listed for and as
// a caller from another organization.
func checkLinkedDocument(t *testing.T, h http.Handler, owner, other, href, listed string) {
	t.Helper()

	got := get(t, h, owner, href)
	if got.Code != http.StatusOK {
		t.Errorf("%s returned %d to the caller the export gave it to", href, got.Code)
		return
	}
	sum := sha256.Sum256(got.Body.Bytes())
	if hex.EncodeToString(sum[:]) != listed {
		t.Errorf("%s does not hash to the sha256 the export listed", href)
	}
	// Read through an fs.FS rooted at the fixtures, so the href cannot climb out.
	want, err := fs.ReadFile(os.DirFS("../../fixtures"), strings.TrimPrefix(href, "/"))
	if err != nil || string(want) != got.Body.String() {
		t.Errorf("%s is not served byte for byte from the fixture (%v)", href, err)
	}
	if w := get(t, h, other, href); w.Code != http.StatusNotFound {
		t.Errorf("%s returned %d to another organization, want 404", href, w.Code)
	}
}
