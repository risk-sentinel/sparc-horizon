package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"testing"
)

// The mock computes every response; the goldens are what it must still say
// about the fixtures at their horizon (#120). Chains keyed on a requirement
// shared through a component definition recur in every boundary that
// inherits it, so the golden holds one of them; those are skipped here and
// covered by TestChainFollowsTheSameRule.
func TestLiveResponsesMatchTheGoldens(t *testing.T) {
	s, h := testServer(t)
	fixtures := os.DirFS("../../fixtures")
	compared := 0

	for _, p := range s.personas {
		paths, _ := fs.Glob(fixtures, "api/"+p.ID+"/*/*.json")
		paths = append(paths, "api/"+p.ID+"/next-action.json")
		for _, path := range paths {
			var url string
			base := strings.TrimSuffix(path[strings.LastIndex(path, "/")+1:], ".json")
			switch {
			case strings.Contains(path, "/heat/"):
				url = "/v1/nodes/" + base + "/heat"
			case strings.Contains(path, "/cell/"):
				node, col, _ := strings.Cut(base, "--")
				url = "/v1/nodes/" + node + "/cell?col=" + strings.ToUpper(col)
			case strings.HasSuffix(path, "next-action.json"):
				url = "/v1/nodes/" + p.NodeUUID + "/next-action"
			default:
				continue
			}
			compareGolden(t, h, fixtures, p.ID, path, url)
			compared++
		}
	}

	compared += compareChains(t, s, h, fixtures)
	if compared < 20 {
		t.Fatalf("compared %d goldens; the walk is not finding them", compared)
	}
	t.Logf("compared %d goldens with live responses", compared)
}

// compareChains compares each NIST-control chain golden as a persona who can
// see it.
func compareChains(t *testing.T, s *server, h http.Handler, fixtures fs.FS) int {
	t.Helper()
	compared := 0
	chains, _ := fs.Glob(fixtures, "api/chain/*.json")
	for _, path := range chains {
		var c struct {
			Component string `json:"component"`
			ControlID string `json:"controlId"`
		}
		raw, _ := fs.ReadFile(fixtures, path)
		if json.Unmarshal(raw, &c) != nil || strings.ToLower(c.ControlID) != c.ControlID {
			continue // a Security Hub requirement, shared across boundaries
		}
		for _, p := range s.personas {
			if s.view.Authorizer().Visible(p.PartyUUID, c.Component) {
				uuid := strings.TrimSuffix(path[strings.LastIndex(path, "/")+1:], ".json")
				compareGolden(t, h, fixtures, p.ID, path, "/v1/controls/"+uuid+"/chain")
				compared++
				break
			}
		}
	}
	return compared
}

func compareGolden(t *testing.T, h http.Handler, fixtures fs.FS, persona, path, url string) {
	t.Helper()
	raw, err := fs.ReadFile(fixtures, path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	w := get(t, h, persona, url)
	if w.Code != http.StatusOK {
		t.Errorf("%s as %s: %d, golden %s exists", url, persona, w.Code, path)
		return
	}
	var want, got any
	_ = json.Unmarshal(raw, &want)
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	a, _ := json.Marshal(want)
	b, _ := json.Marshal(got)
	if string(a) != string(b) {
		t.Errorf("%s as %s differs from %s\n got: %s\nwant: %s", url, persona, path, b, a)
	}
}

// The horizon parameter moves the projection; a malformed one is a 400.
func TestHorizonIsHonoured(t *testing.T) {
	s, h := testServer(t)
	so := s.personas[1]
	url := "/v1/nodes/" + so.NodeUUID + "/heat"

	var now, later struct {
		Horizon string   `json:"horizon"`
		Columns []string `json:"columns"`
	}
	_ = json.Unmarshal(get(t, h, so.ID, url).Body.Bytes(), &now)
	_ = json.Unmarshal(get(t, h, so.ID, url+"?horizon=2027-12-31").Body.Bytes(), &later)
	if now.Horizon != "2026-10-15" || later.Horizon != "2027-12-31" {
		t.Fatalf("horizons %q and %q", now.Horizon, later.Horizon)
	}
	if len(later.Columns) <= len(now.Columns) {
		t.Errorf("projecting 14 months on drew %v, no more than %v: expiries should have taken more down", later.Columns, now.Columns)
	}
	if w := get(t, h, so.ID, url+"?horizon=soon"); w.Code != http.StatusBadRequest {
		t.Errorf("horizon=soon returned %d, want 400", w.Code)
	}
}

// Any OSCAL producer's documents: NIST's example chain, with its roles mapped,
// derives a persona and draws a cell that resolves to a chain.
func TestServesNISTsExampleDocuments(t *testing.T) {
	s, err := newServerWith(os.DirFS("../../internal/oscal/testdata/nist"),
		config{horizon: fixtureHorizon.AddDate(-3, -4, 0), aliases: aliasFlag{"owner": "system-owner"}})
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	if len(s.personas) != 1 || s.personas[0].ID != "so-owen-stilskin" {
		t.Fatalf("personas %+v, want the one system owner", s.personas)
	}
	h, p := s.routes(), s.personas[0]
	var heat struct {
		Columns []string `json:"columns"`
	}
	_ = json.Unmarshal(get(t, h, p.PartyUUID, "/v1/nodes/"+p.NodeUUID+"/heat").Body.Bytes(), &heat)
	if len(heat.Columns) == 0 {
		t.Fatal("no drawn column over NIST's assessment results")
	}
	var cell struct {
		Controls []struct {
			Chain string `json:"chain"`
		} `json:"controls"`
	}
	_ = json.Unmarshal(get(t, h, p.ID, "/v1/nodes/"+p.NodeUUID+"/cell?col="+heat.Columns[0]).Body.Bytes(), &cell)
	if len(cell.Controls) == 0 {
		t.Fatal("the drawn column has no controls")
	}
	if w := get(t, h, p.ID, "/v1/controls/"+cell.Controls[0].Chain+"/chain"); w.Code != http.StatusOK {
		t.Errorf("chain %s: %d", cell.Controls[0].Chain, w.Code)
	}
	if w := get(t, h, p.ID, "/v1/export/oscal/"+p.NodeUUID); w.Code != http.StatusOK {
		t.Errorf("export: %d", w.Code)
	}
}
