package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// The mock exists to be built against, so the thing worth testing is the rule
// a client will be written around: a node outside the caller's tree is 404,
// and it is indistinguishable from one that does not exist.
//
// Checked through the router, as HTTP responses. A refusal rule that holds
// only when handlers are called directly is not a refusal rule.

func testServer(t *testing.T) (*server, http.Handler) {
	t.Helper()

	s, err := newServer(os.DirFS("../../fixtures"))
	if err != nil {
		t.Fatalf("newServer: %v — run `go run ./cmd/genfixtures`", err)
	}
	if len(s.personas) < 3 {
		t.Fatalf("%d personas; the fixtures should carry an AO, an SO and an ISO", len(s.personas))
	}
	return s, s.routes()
}

func get(t *testing.T, h http.Handler, persona, path string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, path, nil)
	if persona != "" {
		r.Header.Set(personaHeader, persona)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// The rule, from both sides: a node another organization owns and a node that
// was never minted produce the same status and the same body.
func TestUnseenNodeAndAbsentNodeAreIndistinguishable(t *testing.T) {
	s, h := testServer(t)

	caller := s.personas[1]                          // SO at a boundary
	foreign := s.personas[2].NodeUUID                // a different organization
	absent := "00000000-0000-5000-8000-000000000000" // never existed

	if s.visible[caller.ID][foreign] {
		t.Fatal("premise wrong: the personas overlap, so this proves nothing")
	}

	unseen := get(t, h, caller.ID, "/v1/nodes/"+foreign+"/heat")
	missing := get(t, h, caller.ID, "/v1/nodes/"+absent+"/heat")

	if unseen.Code != http.StatusNotFound {
		t.Errorf("another organization's node returned %d, want 404 — a 403 confirms it exists", unseen.Code)
	}
	if missing.Code != unseen.Code {
		t.Errorf("absent node returned %d, unseen node %d; they must be identical", missing.Code, unseen.Code)
	}
	if unseen.Body.String() != missing.Body.String() {
		t.Errorf("the two refusals differ in body, which is the leak in another form:\n  unseen:  %s\n  missing: %s",
			unseen.Body.String(), missing.Body.String())
	}
}

func TestCallerSeesTheirOwnTree(t *testing.T) {
	s, h := testServer(t)

	for _, p := range s.personas {
		w := get(t, h, p.ID, "/v1/tree")
		if w.Code != http.StatusOK {
			t.Errorf("%s: tree returned %d", p.ID, w.Code)
			continue
		}
		var root struct {
			ID       string `json:"id"`
			NodeType string `json:"nodeType"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &root); err != nil {
			t.Errorf("%s: %v", p.ID, err)
			continue
		}
		if root.ID != p.NodeUUID {
			t.Errorf("%s: tree rooted at %s, persona is at %s", p.ID, root.ID, p.NodeUUID)
		}
		if root.NodeType != p.NodeType {
			t.Errorf("%s: root is a %s, persona is at a %s", p.ID, root.NodeType, p.NodeType)
		}

		if w := get(t, h, p.ID, "/v1/nodes/"+p.NodeUUID+"/heat"); w.Code != http.StatusOK {
			t.Errorf("%s: own heat returned %d", p.ID, w.Code)
		}
		if w := get(t, h, p.ID, "/v1/nodes/"+p.NodeUUID+"/next-action"); w.Code != http.StatusOK {
			t.Errorf("%s: own next-action returned %d", p.ID, w.Code)
		}
	}
}

// The rule applies one level down too: a chain is reached by requirement UUID,
// and a requirement whose component the caller cannot see is refused the same
// way a node is.
func TestChainFollowsTheSameRule(t *testing.T) {
	s, h := testServer(t)
	caller := s.personas[1]

	var mine, theirs string
	entries, err := os.ReadDir("../../fixtures/api/chain")
	if err != nil {
		t.Fatalf("reading chains: %v", err)
	}
	for _, e := range entries {
		var c struct {
			Requirement string `json:"requirement"`
			Component   string `json:"component"`
		}
		if err := s.read("api/chain/"+e.Name(), &c); err != nil {
			continue
		}
		if s.visible[caller.ID][c.Component] {
			mine = c.Requirement
		} else {
			theirs = c.Requirement
		}
	}
	if mine == "" || theirs == "" {
		t.Fatal("needed one chain inside the caller's tree and one outside it")
	}

	if w := get(t, h, caller.ID, "/v1/controls/"+mine+"/chain"); w.Code != http.StatusOK {
		t.Errorf("own chain returned %d", w.Code)
	}
	if w := get(t, h, caller.ID, "/v1/controls/"+theirs+"/chain"); w.Code != http.StatusNotFound {
		t.Errorf("a chain outside the caller's tree returned %d, want 404", w.Code)
	}
	if w := get(t, h, caller.ID, "/v1/controls/not-a-uuid/chain"); w.Code != http.StatusNotFound {
		t.Errorf("a malformed requirement returned %d, want 404", w.Code)
	}
}

func TestWriteEndpointsRefuseRatherThanPretend(t *testing.T) {
	s, h := testServer(t)

	for _, path := range []string{"/v1/attestations", "/v1/decisions", "/v1/whatif"} {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		r.Header.Set(personaHeader, s.personas[0].ID)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		if w.Code != http.StatusNotImplemented {
			t.Errorf("POST %s returned %d, want 501 — a mock that accepts a write and forgets it is worse than one that refuses", path, w.Code)
		}
	}
}

func TestPersonaSelection(t *testing.T) {
	s, h := testServer(t)

	if w := get(t, h, "", "/v1/tree"); w.Code != http.StatusOK {
		t.Errorf("no persona header returned %d; a bare request should still be useful", w.Code)
	}
	if w := get(t, h, "nobody", "/v1/tree"); w.Code != http.StatusUnauthorized {
		t.Errorf("unknown persona returned %d, want 401 rather than a silent fallback", w.Code)
	}
	_ = s
}

func TestCellAndExport(t *testing.T) {
	s, h := testServer(t)
	caller := s.personas[0]

	var heat struct {
		Columns []string `json:"columns"`
	}
	w := get(t, h, caller.ID, "/v1/nodes/"+caller.NodeUUID+"/heat")
	if err := json.Unmarshal(w.Body.Bytes(), &heat); err != nil {
		t.Fatalf("heat: %v", err)
	}
	if len(heat.Columns) == 0 {
		t.Fatal("the persona's heat has no drawn columns, so there is no cell to drill into")
	}

	if w := get(t, h, caller.ID, "/v1/nodes/"+caller.NodeUUID+"/cell?col="+heat.Columns[0]); w.Code != http.StatusOK {
		t.Errorf("cell for a drawn column returned %d", w.Code)
	}
	if w := get(t, h, caller.ID, "/v1/nodes/"+caller.NodeUUID+"/cell?col=../../etc"); w.Code != http.StatusBadRequest {
		t.Errorf("a column that is a path returned %d, want 400", w.Code)
	}
	if w := get(t, h, caller.ID, "/v1/export/oscal/ods-portal"); w.Code != http.StatusOK {
		t.Errorf("export returned %d", w.Code)
	}
	if w := get(t, h, caller.ID, "/v1/export/oscal/no-such-boundary"); w.Code != http.StatusNotFound {
		t.Errorf("export of an unknown boundary returned %d, want 404", w.Code)
	}
}
