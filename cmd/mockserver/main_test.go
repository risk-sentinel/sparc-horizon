package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
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

	// Premise, asserted rather than assumed: if these two personas overlapped,
	// the test below would pass while proving nothing. Asked of internal/authz
	// since #79, which is the same question the server now answers with.
	if s.authz.Visible(caller.PartyUUID, foreign) {
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
		if s.authz.Visible(caller.PartyUUID, c.Component) {
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

// GET /v1/tree is computed from the documents since #79, not served from a
// file — and it must still equal the golden frozen in #61, which is what
// cmd/mockserver promised clients before the rewire.
//
// Compared as data with siblings sorted: the contract leaves `children` order
// unspecified and the goldens carry the fixture generator's table order, which
// no document determines (docs/10-risks-decisions.md).
func TestTreeIsComputedAndStillMatchesTheGolden(t *testing.T) {
	s, h := testServer(t)

	for _, p := range s.personas {
		t.Run(p.ID, func(t *testing.T) {
			raw, err := fs.ReadFile(s.fsys, "api/"+p.ID+"/tree.json")
			if err != nil {
				t.Fatalf("golden: %v", err)
			}
			var want, got any
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("golden: %v", err)
			}

			res := get(t, h, p.ID, "/v1/tree")
			if res.Code != http.StatusOK {
				t.Fatalf("GET /v1/tree returned %d", res.Code)
			}
			if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
				t.Fatalf("response: %v", err)
			}

			if !sameTree(want, got) {
				t.Errorf("computed tree differs from the frozen golden\n got: %s\nwant: %s",
					res.Body.String(), raw)
			}
		})
	}
}

// The mock cannot answer anything without the documents, so it must refuse to
// start rather than serve a federation in which every node is invisible —
// which is indistinguishable from a working mock that simply denies you.
func TestServerRefusesWithoutTheOSCALFixtures(t *testing.T) {
	only := os.DirFS("../../fixtures/api")
	if _, err := newServer(only); err == nil {
		t.Error("newServer started with no SSPs; every response would be a 404 and it would look healthy")
	}
}

func sameTree(a, b any) bool {
	x, _ := json.Marshal(normaliseTree(a))
	y, _ := json.Marshal(normaliseTree(b))
	return string(x) == string(y)
}

func normaliseTree(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	kids, ok := m["children"].([]any)
	if !ok {
		return m
	}
	for i := range kids {
		kids[i] = normaliseTree(kids[i])
	}
	sort.Slice(kids, func(i, j int) bool {
		l, _ := kids[i].(map[string]any)["id"].(string)
		r, _ := kids[j].(map[string]any)["id"].(string)
		return l < r
	})
	m["children"] = kids
	return m
}

// hideFS serves a filesystem with one path removed, so a test can prove a
// handler does not read it.
type hideFS struct {
	fs.FS
	hidden string
}

func (h hideFS) Open(name string) (fs.File, error) {
	if name == h.hidden {
		return nil, fs.ErrNotExist
	}
	return h.FS.Open(name)
}

func (h hideFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(h.FS, name)
}

// The proof that /v1/tree is COMPUTED rather than served.
//
// Comparing the response to the golden cannot show this — they agree, which is
// the point, so serving the file would pass too. Hiding the golden can: if the
// handler still answers correctly with `api/<persona>/tree.json` unreadable,
// it cannot have read it.
func TestTreeIsComputedNotRead(t *testing.T) {
	const who = "so-ods-portal"

	fixtures := os.DirFS("../../fixtures")
	raw, err := fs.ReadFile(fixtures, "api/"+who+"/tree.json")
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	var want any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("golden: %v", err)
	}

	s, err := newServer(hideFS{FS: fixtures, hidden: "api/" + who + "/tree.json"})
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}

	res := get(t, s.routes(), who, "/v1/tree")
	if res.Code != http.StatusOK {
		t.Fatalf("GET /v1/tree returned %d with the golden hidden — it is being read, not computed", res.Code)
	}
	var got any
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v", err)
	}
	if !sameTree(want, got) {
		t.Errorf("computed tree differs from the golden it must still match\n got: %s\nwant: %s",
			res.Body.String(), raw)
	}
}
