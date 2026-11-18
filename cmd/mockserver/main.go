// Command mockserver serves API v0 over a folder of OSCAL documents.
//
// Point it at the fixture federation or at your own SSP, assessment plan,
// assessment results and POA&M documents, from SPARC or any other OSCAL
// producer: every response is computed from the documents at the horizon the
// request asks for, by internal/view (#120). The goldens in `fixtures/api/` are
// the contract's examples and the regression baseline, not what it reads.
//
// # What it does not do
//
// It does not persist anything and has no ledger: the documents are read once
// at startup. It does not authenticate: a header picks the persona. It answers
// the write endpoints with 501 rather than pretending, because a mock that
// accepts an attestation and forgets it is worse than one that refuses.
//
// What it does enforce is the contract's refusal rule: a node outside the
// caller's tree is **404**, indistinguishable from one that does not exist.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/risk-sentinel/sparc-horizon/internal/tree"
	"github.com/risk-sentinel/sparc-horizon/internal/view"
)

const (
	personaHeader = "X-Horizon-Persona"
	contentType   = "Content-Type"
	mediaJSON     = "application/json"
)

type server struct {
	view     *view.View
	personas []view.Persona
	// horizon is the date a request without one projects to.
	horizon time.Time
	// static, when set, is served at `/`: the HUD (web/dist), so one process
	// is the whole working model. The API stays under `/v1/`.
	static fs.FS
	// demo, when set, is served at `/demo/`: the static design demos, which
	// carry their own synthetic data and inline code, so a looser policy.
	demo fs.FS
}

// Content-Security-Policy per surface (SC-18): the browser runs only the
// HUD's own code. The HUD loads one script and one stylesheet from its own
// origin and talks only to it; every document string it draws is untrusted,
// so nothing inline is allowed. The API answers JSON that no browser should
// execute or frame. The demos are single files with inline code and Google
// Fonts, and are throwaway (#122).
const (
	cspHUD  = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	cspAPI  = "default-src 'none'; frame-ancestors 'none'"
	cspDemo = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
)

// config is what main reads from flags.
type config struct {
	horizon time.Time
	aliases map[string]tree.Role
}

// Path components are matched rather than trusted.
var (
	uuidRe = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
	colRe  = regexp.MustCompile(`^[A-Za-z0-9.-]{1,16}$`)
)

// aliasFlag collects -role-alias doc-role=contract-role.
type aliasFlag map[string]tree.Role

func (a aliasFlag) String() string { return fmt.Sprint(map[string]tree.Role(a)) }

func (a aliasFlag) Set(s string) error {
	from, to, ok := strings.Cut(s, "=")
	role := tree.Role(to)
	if !ok || from == "" || (role != tree.AuthorizingOfficial && role != tree.SystemOwner && role != tree.ISSO) {
		return errors.New("want <document role-id>=<authorizing-official|system-owner|information-system-security-officer>")
	}
	a[from] = role
	return nil
}

func main() {
	dir := flag.String("dir", "fixtures", "folder of OSCAL documents to serve")
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	static := flag.String("static", "", "directory to serve at / (the HUD, web/dist); off when empty")
	demo := flag.String("demo", "", "directory to serve at /demo/ (the static design demos); off when empty")
	horizon := flag.String("horizon", "", "default horizon, YYYY-MM-DD; today when empty")
	aliases := aliasFlag{}
	flag.Var(aliases, "role-alias", "map a document role-id onto a contract role, e.g. owner=system-owner (repeatable)")
	flag.Parse()

	cfg := config{horizon: today(), aliases: aliases}
	if *horizon != "" {
		t, err := view.ParseHorizon(*horizon, cfg.horizon)
		if err != nil {
			log.Fatalf("mockserver: -horizon: %v", err)
		}
		cfg.horizon = t
	}

	// Read through an os.Root, so nothing under -dir reaches outside it.
	root, err := os.OpenRoot(*dir)
	if err != nil {
		log.Fatalf("mockserver: -dir: %v", err)
	}
	s, err := newServerWith(root.FS(), cfg)
	if err != nil {
		log.Fatalf("mockserver: %v", err)
	}
	if *static != "" {
		if _, err := os.Stat(*static); err != nil {
			log.Fatalf("mockserver: -static: %v", err)
		}
		s.static = os.DirFS(*static)
		log.Printf("mockserver: serving %s at /", *static)
	}
	if *demo != "" {
		if _, err := os.Stat(*demo); err != nil {
			log.Fatalf("mockserver: -demo: %v", err)
		}
		s.demo = os.DirFS(*demo)
		log.Printf("mockserver: serving %s at /demo/", *demo)
	}

	log.Printf("mockserver: %s", s.view.Summary())
	for _, p := range s.view.Problems() {
		log.Printf("  problem: %s: %s", p.Path, p.Detail)
	}
	log.Printf("mockserver: %d personas, default horizon %s, serving %s on http://%s",
		len(s.personas), s.horizon.Format(view.DateLayout), *dir, *addr)
	for _, p := range s.personas {
		log.Printf("  %s %-34s %s", personaHeader+":", p.ID, p.Name)
	}
	if len(s.personas) == 0 {
		log.Printf("mockserver: no party holds authorizing-official, system-owner or information-system-security-officer; " +
			"map the documents' roles with -role-alias (see docs/testing.md)")
	}
	srv := &http.Server{Addr: *addr, Handler: s.routes(), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func today() time.Time {
	y, m, d := time.Now().UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// routes is separate from main so the contract's refusal rule can be tested
// through the router rather than by calling handlers directly: the rule is
// only worth anything as an HTTP response.
func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/tree", s.tree)
	mux.HandleFunc("GET /v1/nodes/{id}/heat", s.heat)
	mux.HandleFunc("GET /v1/nodes/{id}/cell", s.cell)
	mux.HandleFunc("GET /v1/nodes/{id}/next-action", s.nextAction)
	mux.HandleFunc("GET /v1/controls/{uuid}/chain", s.chain)
	mux.HandleFunc("GET /v1/export/oscal/{boundary}", s.export)
	// Not a contract path: the href an export returns has to resolve, and the
	// mock has nowhere else to serve the documents from.
	mux.HandleFunc("GET /oscal/{name...}", s.document)
	// Not a contract path either: the HUD's persona picker. A real server
	// knows the caller from the token; the mock has to offer a choice.
	mux.HandleFunc("GET /mock/personas", s.personaList)
	mux.HandleFunc("/v1/", s.notImplemented)
	mux.HandleFunc("GET /healthz", healthz)
	if s.demo != nil {
		mux.Handle("/demo/", http.StripPrefix("/demo", files(s.demo, cspDemo)))
	}
	if s.static != nil {
		// Registered for every method and narrowed in files: "GET /" would
		// conflict with the method-less "/v1/" above, since each pattern is
		// more specific than the other in one dimension.
		mux.Handle("/", files(s.static, cspHUD))
	}
	return secureHeaders(mux)
}

// secureHeaders sets the API's policy on every response; files replaces it
// with the policy for the surface it serves.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", cspAPI)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// files serves a directory read-only under the given policy.
func files(fsys fs.FS, csp string) http.Handler {
	fileServer := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Static files are read-only.")
			return
		}
		w.Header().Set("Content-Security-Policy", csp)
		fileServer.ServeHTTP(w, r)
	})
}

// personaList answers the mock's callers, in the shape of personas.json.
func (s *server) personaList(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(contentType, mediaJSON)
	_ = json.NewEncoder(w).Encode(s.personas)
}

// healthz answers liveness only: newServer refused to start without an SSP,
// so a process that is up is a process that loaded one.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(contentType, mediaJSON)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func newServer(fsys fs.FS) (*server, error) {
	return newServerWith(fsys, config{horizon: today()})
}

func newServerWith(fsys fs.FS, cfg config) (*server, error) {
	set, err := view.LoadFS(fsys)
	if err != nil {
		return nil, fmt.Errorf("reading the documents: %w", err)
	}
	v, err := view.New(set, view.Options{RoleAliases: cfg.aliases})
	if err != nil {
		// Starting anyway would serve a federation of nothing: every node
		// invisible and every response a 404, which is what a working mock
		// with no data looks like.
		return nil, fmt.Errorf("%w (an SSP is required; run `go run ./cmd/genfixtures` for the fixtures)", err)
	}
	s := &server{view: v, horizon: cfg.horizon}

	// A personas file names the callers; without one they come from the
	// documents' role holders.
	if s.personas, err = readPersonas(fsys); err != nil {
		return nil, err
	}
	if s.personas == nil {
		s.personas = v.Personas()
	}
	for _, p := range s.personas {
		if p.PartyUUID == "" {
			return nil, fmt.Errorf("persona %q has no party-uuid, so it cannot be authorized", p.ID)
		}
		if v.Authorizer().Subtree(p.PartyUUID) == nil {
			return nil, fmt.Errorf("persona %q holds no role anywhere in the documents", p.ID)
		}
	}
	return s, nil
}

func readPersonas(fsys fs.FS) ([]view.Persona, error) {
	for _, name := range []string{"api/personas.json", "personas.json"} {
		b, err := fs.ReadFile(fsys, name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var out []view.Persona
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return out, nil
	}
	return nil, nil
}

// who resolves the caller, by persona id or party UUID. Defaulting to the
// first persona keeps a bare curl useful; naming an unknown one is an error
// rather than a silent fallback.
func (s *server) who(r *http.Request) (view.Persona, bool) {
	want := r.Header.Get(personaHeader)
	if want == "" && len(s.personas) > 0 {
		return s.personas[0], true
	}
	for _, p := range s.personas {
		if p.ID == want || (want != "" && strings.EqualFold(p.PartyUUID, want)) {
			return p, true
		}
	}
	return view.Persona{}, false
}

func (s *server) caller(w http.ResponseWriter, r *http.Request) (view.Persona, bool) {
	p, ok := s.who(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unknown_persona",
			"Set "+personaHeader+" to a persona id or party UUID; the server logs them at startup.")
		return view.Persona{}, false
	}
	return p, true
}

// horizonOf reads the request's horizon, answering 400 for a malformed one.
func (s *server) horizonOf(w http.ResponseWriter, r *http.Request) (time.Time, bool) {
	t, err := view.ParseHorizon(r.URL.Query().Get("horizon"), s.horizon)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_horizon", err.Error())
		return time.Time{}, false
	}
	return t, true
}

// request resolves the caller, the node id and the horizon.
func (s *server) request(w http.ResponseWriter, r *http.Request) (view.Persona, string, time.Time, bool) {
	p, ok := s.caller(w, r)
	if !ok {
		return view.Persona{}, "", time.Time{}, false
	}
	at, ok := s.horizonOf(w, r)
	if !ok {
		return view.Persona{}, "", time.Time{}, false
	}
	return p, r.PathValue("id"), at, true
}

func send(w http.ResponseWriter, v any, ok bool) {
	if !ok {
		notFound(w)
		return
	}
	w.Header().Set(contentType, mediaJSON)
	_ = json.NewEncoder(w).Encode(v)
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

// tree is the caller's tree: every node they hold a role on or beneath.
func (s *server) tree(w http.ResponseWriter, r *http.Request) {
	p, ok := s.caller(w, r)
	if !ok {
		return
	}
	sub := s.view.Authorizer().Subtree(p.PartyUUID)
	send(w, sub, sub != nil)
}

func (s *server) heat(w http.ResponseWriter, r *http.Request) {
	if p, id, at, ok := s.request(w, r); ok {
		h, found := s.view.Heat(p.PartyUUID, id, at)
		send(w, h, found)
	}
}

func (s *server) cell(w http.ResponseWriter, r *http.Request) {
	p, id, at, ok := s.request(w, r)
	if !ok {
		return
	}
	col := r.URL.Query().Get("col")
	if !colRe.MatchString(col) {
		writeError(w, http.StatusBadRequest, "bad_column", "col must be a column from the heat response.")
		return
	}
	c, found := s.view.Cell(p.PartyUUID, id, strings.ToUpper(col), at)
	send(w, c, found)
}

func (s *server) nextAction(w http.ResponseWriter, r *http.Request) {
	if p, id, at, ok := s.request(w, r); ok {
		n, found := s.view.NextAction(p.PartyUUID, id, at)
		send(w, n, found)
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
	c, found := s.view.Chain(p.PartyUUID, uuid)
	send(w, c, found)
}

// export lists a boundary's documents, by slug or SSP UUID, refused the way a
// node is.
func (s *server) export(w http.ResponseWriter, r *http.Request) {
	p, ok := s.caller(w, r)
	if !ok {
		return
	}
	e, found := s.view.Export(p.PartyUUID, r.PathValue("boundary"))
	send(w, e, found)
}

// document serves a document an export links to, byte for byte, so the
// sha256 the export listed verifies against what arrives. Only documents an
// export lists resolve, and only for a caller who can see their boundary.
func (s *server) document(w http.ResponseWriter, r *http.Request) {
	p, ok := s.caller(w, r)
	if !ok {
		return
	}
	b, found := s.view.Document(p.PartyUUID, "/oscal/"+r.PathValue("name"))
	if !found {
		notFound(w)
		return
	}
	w.Header().Set(contentType, mediaJSON)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(b))
}

func (s *server) notImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented",
		"The mock serves the read endpoints. "+r.Method+" "+r.URL.Path+
			" is in the frozen contract and is implemented by the service, not here.")
}
