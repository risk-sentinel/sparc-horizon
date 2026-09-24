// Command mockserver serves API v0 over the fixture federation.
//
// It exists so P3 can build the HUD and P1 can shape its client before either
// has a service to talk to. The responses are the goldens in `fixtures/api/`,
// derived by parsing the OSCAL the fixture generator emitted — so what the
// mock serves recomputes from exported documents, which is the property the
// real service has to keep.
//
// # What it does not do
//
// It does not project. Cell states were computed once, at generation time,
// for one horizon; the `horizon` parameter is accepted and ignored. It does
// not authenticate — a header picks the persona. It answers the write
// endpoints with 501 rather than pretending, because a mock that accepts an
// attestation and forgets it is worse than one that refuses.
//
// What it does enforce is the contract's refusal rule: a node outside the
// caller's tree is **404**, indistinguishable from one that does not exist.
// That is the part a client will be written against, and getting it wrong in
// the mock means getting it wrong in the client.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"

	"github.com/risk-sentinel/sparc-horizon/internal/authz"
	"github.com/risk-sentinel/sparc-horizon/internal/tree"
)

const (
	personaHeader = "X-Horizon-Persona"
	contentType   = "Content-Type"
	mediaJSON     = "application/json"
)

type persona struct {
	ID string `json:"id"`
	// PartyUUID is the caller. internal/authz takes a party uuid, because how
	// an OIDC subject maps to one is still an open decision
	// (docs/10-risks-decisions.md); the persona header stands in for the login
	// the mock deliberately does not perform.
	PartyUUID string `json:"party-uuid"`
	Role      string `json:"role"`
	NodeUUID  string `json:"node-uuid"`
	NodeType  string `json:"node-type"`
	Name      string `json:"name"`
}

type server struct {
	fsys     fs.FS
	personas []persona
	// authz decides every refusal. It was a map flattened out of the committed
	// goldens until #79 — which meant the mock sided with the generator rather
	// than with the rule, and could not show that a caller's tree is COMPUTED
	// from their bindings rather than stored.
	authz *authz.Authorizer
}

// Path components are matched rather than trusted: a UUID or a column, and
// nothing that could climb out of the fixtures directory.
var (
	uuidRe = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
	colRe  = regexp.MustCompile(`^[A-Za-z0-9.-]{1,16}$`)
)

func main() {
	dir := flag.String("dir", "fixtures", "fixtures directory to serve")
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	flag.Parse()

	s, err := newServer(os.DirFS(*dir))
	if err != nil {
		log.Fatalf("mockserver: %v", err)
	}

	mux := s.routes()

	log.Printf("mockserver: %d personas, serving %s on http://%s", len(s.personas), *dir, *addr)
	for _, p := range s.personas {
		log.Printf("  %s %-34s %s", personaHeader+":", p.ID, p.Name)
	}
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * 1e9}
	log.Fatal(srv.ListenAndServe())
}

// routes is separate from main so the contract's refusal rule can be tested
// through the router rather than by calling handlers directly — the rule is
// only worth anything as an HTTP response.
func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/tree", s.tree)
	mux.HandleFunc("GET /v1/nodes/{id}/heat", s.heat)
	mux.HandleFunc("GET /v1/nodes/{id}/cell", s.cell)
	mux.HandleFunc("GET /v1/nodes/{id}/next-action", s.nextAction)
	mux.HandleFunc("GET /v1/controls/{uuid}/chain", s.chain)
	mux.HandleFunc("GET /v1/export/oscal/{boundary}", s.export)
	mux.HandleFunc("/v1/", s.notImplemented)
	return mux
}

func newServer(fsys fs.FS) (*server, error) {
	s := &server{fsys: fsys}
	if err := s.read("api/personas.json", &s.personas); err != nil {
		return nil, fmt.Errorf("no personas: %w (run `go run ./cmd/genfixtures`)", err)
	}

	a, err := buildAuthz(fsys)
	if err != nil {
		// Starting anyway would serve a federation of nothing: every node
		// would be invisible and every response a 404, which is exactly what
		// a working mock with no data looks like.
		return nil, err
	}
	s.authz = a

	for _, p := range s.personas {
		if p.PartyUUID == "" {
			return nil, fmt.Errorf("persona %q has no party-uuid, so it cannot be authorized", p.ID)
		}
		if s.authz.Subtree(p.PartyUUID) == nil {
			return nil, fmt.Errorf("persona %q holds no role anywhere in the fixture federation", p.ID)
		}
	}
	return s, nil
}

// buildAuthz assembles the tree from the OSCAL the fixtures carry, then indexes
// it for authorization — the same two packages the service will use.
//
// The mock reads the DOCUMENTS, not the API goldens it serves. That is the
// property worth demonstrating: a caller's tree is derived from what the
// documents say, so a client written against this mock is written against the
// behaviour the service has.
func buildAuthz(fsys fs.FS) (*authz.Authorizer, error) {
	names, err := fs.Glob(fsys, "oscal/ssp-*.json")
	if err != nil {
		return nil, fmt.Errorf("looking for SSPs: %w", err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no SSP fixtures under oscal/ (run `go run ./cmd/genfixtures`)")
	}
	sort.Strings(names)

	ssps := make([]*oscal.SystemSecurityPlan, 0, len(names))
	for _, n := range names {
		b, err := fs.ReadFile(fsys, n)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		var doc oscal.OscalCompleteSchema
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		if doc.SystemSecurityPlan == nil {
			return nil, fmt.Errorf("%s is not a system-security-plan", n)
		}
		ssps = append(ssps, doc.SystemSecurityPlan)
	}

	res, err := tree.Build(ssps)
	if err != nil {
		return nil, fmt.Errorf("building the tree: %w", err)
	}
	// Findings are reported, not fatal: the fixtures are expected to be clean,
	// and a mock that refused to start on one orphaned party would be useless
	// for exactly the malformed-document work it should help with.
	for _, f := range res.Findings {
		log.Printf("mockserver: tree finding %s (%s): %s", f.Kind, f.Source, f.Detail)
	}
	return authz.New(res)
}

func (s *server) read(name string, into any) error {
	b, err := fs.ReadFile(s.fsys, name)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, into)
}

// who resolves the caller. Defaulting to the first persona keeps a bare curl
// useful; naming an unknown one is an error rather than a silent fallback.
func (s *server) who(r *http.Request) (persona, bool) {
	want := r.Header.Get(personaHeader)
	if want == "" {
		return s.personas[0], true
	}
	for _, p := range s.personas {
		if p.ID == want {
			return p, true
		}
	}
	return persona{}, false
}

// serve writes a golden, or the contract's 404 when it is not there.
//
// The two cases are deliberately one code path: "no such node" and "not yours"
// produce the same response, so the server cannot accidentally distinguish
// them.
func (s *server) serve(w http.ResponseWriter, name string) {
	b, err := fs.ReadFile(s.fsys, name)
	if err != nil {
		notFound(w)
		return
	}
	w.Header().Set(contentType, mediaJSON)
	_, _ = w.Write(b)
}

func notFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not_found",
		"No such node, or you hold no role on it. These are deliberately indistinguishable.")
}

func writeError(w http.ResponseWriter, code int, err, detail string) {
	w.Header().Set(contentType, mediaJSON)
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err, "detail": detail})
}

func (s *server) caller(w http.ResponseWriter, r *http.Request) (persona, bool) {
	p, ok := s.who(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unknown_persona",
			"Set "+personaHeader+" to a persona from fixtures/api/personas.json.")
		return persona{}, false
	}
	return p, true
}

// node resolves a node id and applies the refusal rule in one place.
func (s *server) node(w http.ResponseWriter, r *http.Request) (persona, string, bool) {
	p, ok := s.caller(w, r)
	if !ok {
		return persona{}, "", false
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) || !s.authz.Visible(p.PartyUUID, id) {
		notFound(w)
		return persona{}, "", false
	}
	return p, id, true
}

// tree is COMPUTED, not served. Every other read endpoint returns a golden,
// because the mock does not project; this one is the exception because the
// caller's tree is the one thing authorization actually decides.
func (s *server) tree(w http.ResponseWriter, r *http.Request) {
	p, ok := s.caller(w, r)
	if !ok {
		return
	}
	sub := s.authz.Subtree(p.PartyUUID)
	if sub == nil {
		// A caller holding nothing is refused the same way a caller asking
		// about somebody else's node is.
		notFound(w)
		return
	}
	w.Header().Set(contentType, mediaJSON)
	_ = json.NewEncoder(w).Encode(sub)
}

func (s *server) heat(w http.ResponseWriter, r *http.Request) {
	if p, id, ok := s.node(w, r); ok {
		s.serve(w, path.Join("api", p.ID, "heat", id+".json"))
	}
}

func (s *server) cell(w http.ResponseWriter, r *http.Request) {
	p, id, ok := s.node(w, r)
	if !ok {
		return
	}
	col := r.URL.Query().Get("col")
	if !colRe.MatchString(col) {
		writeError(w, http.StatusBadRequest, "bad_column", "col must be a column from the heat response.")
		return
	}
	s.serve(w, path.Join("api", p.ID, "cell", id+"--"+strings.ToLower(col)+".json"))
}

func (s *server) nextAction(w http.ResponseWriter, r *http.Request) {
	if p, _, ok := s.node(w, r); ok {
		s.serve(w, path.Join("api", p.ID, "next-action.json"))
	}
}

// chain applies the same rule one level down: a requirement is visible when
// its component is.
func (s *server) chain(w http.ResponseWriter, r *http.Request) {
	p, ok := s.caller(w, r)
	if !ok {
		return
	}
	uuid := r.PathValue("uuid")
	if !uuidRe.MatchString(uuid) {
		notFound(w)
		return
	}
	var c struct {
		Component string `json:"component"`
	}
	// The chain names the component it belongs to, and since #74 a system node
	// IS that component — so the same visibility rule answers it.
	if err := s.read(path.Join("api", "chain", uuid+".json"), &c); err != nil || !s.authz.Visible(p.PartyUUID, c.Component) {
		notFound(w)
		return
	}
	s.serve(w, path.Join("api", "chain", uuid+".json"))
}

// export hands back the OSCAL itself, which is what the recompute audit reads.
func (s *server) export(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.caller(w, r); !ok {
		return
	}
	boundary := r.PathValue("boundary")
	if !colRe.MatchString(boundary) && !uuidRe.MatchString(boundary) {
		notFound(w)
		return
	}
	docs := []map[string]string{}
	for model, prefix := range map[string]string{
		"system-security-plan":          "ssp-",
		"assessment-results":            "ar-",
		"plan-of-action-and-milestones": "poam-",
	} {
		name := path.Join("oscal", prefix+boundary+".json")
		if _, err := fs.Stat(s.fsys, name); err == nil {
			docs = append(docs, map[string]string{"model": model, "href": "/" + name})
		}
	}
	if len(docs) == 0 {
		notFound(w)
		return
	}
	w.Header().Set(contentType, mediaJSON)
	_ = json.NewEncoder(w).Encode(map[string]any{"boundary": boundary, "documents": docs})
}

func (s *server) notImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented",
		"The mock serves the read endpoints from fixtures. "+r.Method+" "+r.URL.Path+
			" is in the frozen contract and is implemented by the service, not here.")
}
