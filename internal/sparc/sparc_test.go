package sparc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const token = "sparc_0123456789abcdef"

// fakeSPARC plays SPARC's /api/v1 contract as read from its source (#124): the
// list envelope, the export route with format and validate, a strong ETag over
// the bytes, and SPARC's `{error, details}` refusals.
type fakeSPARC struct {
	mu       sync.Mutex
	docs     map[string][]listRow // collection → rows
	bodies   map[string]string    // "<collection>/<key>" → export body
	status   map[string]int       // "<collection>/<key>" → forced status
	sent     int                  // export bodies sent with 200
	notMod   int                  // 304s
	lastAuth string
	lastQ    string
	// countSkew and pagesSkew add to meta.count and meta.pages on pages after
	// the first: a list that changed while it was being walked.
	countSkew, pagesSkew int
}

func newFake() *fakeSPARC {
	return &fakeSPARC{docs: map[string][]listRow{}, bodies: map[string]string{}, status: map[string]int{}}
}

func (f *fakeSPARC) add(k Kind, slug, body string) {
	col := k.collection()
	n := len(f.docs[col]) + 1
	f.docs[col] = append(f.docs[col], listRow{ID: json.Number(strconv.Itoa(n)), Slug: slug, UUID: fmt.Sprintf("uuid-%s", slug)})
	f.bodies[col+"/"+slug] = body
}

func oscalBody(k Kind, uuid string) string {
	return fmt.Sprintf(`{%q:{"uuid":%q}}`, k.root(), uuid)
}

func (f *fakeSPARC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastAuth = r.Header.Get("Authorization")
	f.lastQ = r.URL.RawQuery
	if f.lastAuth != "Bearer "+token {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Missing or invalid Authorization header. Use: Authorization: Bearer <token>"}`))
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/"), "/")
	switch len(parts) {
	case 1:
		f.list(w, r, parts[0])
	case 3:
		f.export(w, r, parts[0], parts[1])
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeSPARC) list(w http.ResponseWriter, r *http.Request, col string) {
	rows := f.docs[col]
	per, _ := strconv.Atoi(r.URL.Query().Get("items"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pages := max(1, (len(rows)+per-1)/per)
	lo, hi := min((page-1)*per, len(rows)), min(page*per, len(rows))
	count := len(rows)
	if page > 1 {
		count += f.countSkew
		pages += f.pagesSkew
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": rows[lo:hi],
		"meta": map[string]int{"page": page, "pages": pages, "count": count, "items": per},
	})
}

func (f *fakeSPARC) export(w http.ResponseWriter, r *http.Request, col, key string) {
	if code := f.status[col+"/"+key]; code != 0 {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"error":"The system security plan does not conform to the OSCAL schema","details":["/metadata: missing title"],"hint":"Re-request with validate=false to export it anyway"}`))
		return
	}
	body, ok := f.bodies[col+"/"+key]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Not found"}`))
		return
	}
	tag := `"` + digest([]byte(body))[:16] + `"`
	if r.Header.Get("If-None-Match") == tag {
		f.notMod++
		w.WriteHeader(http.StatusNotModified)
		return
	}
	f.sent++
	w.Header().Set("ETag", tag)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

// start serves fake over TLS and returns a client that trusts it through the
// unexported test hook.
func start(t *testing.T, h http.Handler, mutate ...func(*Config)) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	cfg := Config{BaseURL: srv.URL, Token: token, roots: srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs}
	for _, m := range mutate {
		m(&cfg)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c, srv
}

func TestPlainHTTPIsRefusedOnEveryHost(t *testing.T) {
	for _, u := range []string{
		"http://sparc.example.gov",
		"http://127.0.0.1:3000",
		"http://localhost",
		"HTTP://sparc.example.gov",
		"sparc.example.gov",
		"ftp://sparc.example.gov",
	} {
		if _, err := New(Config{BaseURL: u, Token: token}); !errors.Is(err, ErrInsecureURL) {
			t.Errorf("%s: got %v, want ErrInsecureURL", u, err)
		}
	}
}

func TestUnusableBaseURLsAreRefused(t *testing.T) {
	for _, u := range []string{
		"https://",
		"https://user:pass@sparc.example.gov",
		"https://sparc.example.gov/?x=1",
		"https://sparc.example.gov/#frag",
	} {
		if _, err := New(Config{BaseURL: u, Token: token}); !errors.Is(err, ErrBadURL) {
			t.Errorf("%s: got %v, want ErrBadURL", u, err)
		}
	}
}

func TestTokenIsRequiredAndCannotSmuggleAHeader(t *testing.T) {
	for _, tok := range []string{"", "sparc_abc\r\nX-Evil: 1", "sparc abc", "sparc_\x00"} {
		if _, err := New(Config{BaseURL: "https://sparc.example.gov", Token: tok}); !errors.Is(err, ErrNoToken) {
			t.Errorf("%q: got %v, want ErrNoToken", tok, err)
		}
	}
}

// The system roots are what the shipped client trusts, so a server signed by
// any other CA is refused. This is the test that the hook is the ONLY way the
// test server became trusted.
func TestAnUntrustedCertificateIsRefused(t *testing.T) {
	srv := httptest.NewTLSServer(newFake())
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL, Token: token})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.List(context.Background(), SSP)
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("got %v, want a certificate verification failure", err)
	}
}

// Go's client already refuses below TLS 1.2 by default, so the handshake test
// below passes with or without the explicit floor. The floor is stated in the
// configuration so a toolchain default cannot lower it; this asserts it is.
func TestTheTLSFloorIsConfigured(t *testing.T) {
	c, err := New(Config{BaseURL: "https://sparc.example.gov", Token: token})
	if err != nil {
		t.Fatal(err)
	}
	cfg := c.http.Transport.(*http.Transport).TLSClientConfig
	if cfg.MinVersion < tls.VersionTLS12 || cfg.InsecureSkipVerify || cfg.RootCAs != nil {
		t.Fatalf("TLS config min=%#x skip=%v roots=%v", cfg.MinVersion, cfg.InsecureSkipVerify, cfg.RootCAs)
	}
}

func TestListWalksEveryPage(t *testing.T) {
	f := newFake()
	for i := range 250 {
		f.add(SSP, fmt.Sprintf("ssp-%03d", i), oscalBody(SSP, "u"))
	}
	c, _ := start(t, f)
	docs, err := c.List(context.Background(), SSP)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 250 {
		t.Fatalf("listed %d, want 250 across three pages", len(docs))
	}
	if docs[249].Slug != "ssp-249" || docs[0].ID != "1" {
		t.Errorf("rows out of order or mis-decoded: first %+v last %+v", docs[0], docs[249])
	}
	if f.lastAuth != "Bearer "+token {
		t.Errorf("Authorization was %q", f.lastAuth)
	}
}

func TestEmptyListIsNotAnError(t *testing.T) {
	c, _ := start(t, newFake())
	docs, err := c.List(context.Background(), POAM)
	if err != nil || len(docs) != 0 {
		t.Fatalf("got %d docs, %v", len(docs), err)
	}
}

func TestAListThatChangesMidWalkIsRefused(t *testing.T) {
	f := newFake()
	for i := range 150 {
		f.add(SAR, fmt.Sprintf("sar-%d", i), oscalBody(SAR, "u"))
	}
	f.countSkew = 1
	c, _ := start(t, f)
	if _, err := c.List(context.Background(), SAR); !errors.Is(err, ErrList) {
		t.Fatalf("got %v, want ErrList", err)
	}
}

// A page count that grows mid-walk with the total unchanged is caught only by
// the per-page comparison: the extra page is empty, so the final total agrees.
func TestAPageCountThatChangesMidWalkIsRefused(t *testing.T) {
	f := newFake()
	for i := range 150 {
		f.add(SAR, fmt.Sprintf("sar-%d", i), oscalBody(SAR, "u"))
	}
	f.pagesSkew = 1
	c, _ := start(t, f)
	if _, err := c.List(context.Background(), SAR); !errors.Is(err, ErrList) {
		t.Fatalf("got %v, want ErrList", err)
	}
}

func TestAListWithoutMetaIsRefused(t *testing.T) {
	c, _ := start(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	if _, err := c.List(context.Background(), SSP); !errors.Is(err, ErrList) {
		t.Fatalf("got %v, want ErrList", err)
	}
}

func TestAListRowNamingAPathIsRefused(t *testing.T) {
	for _, slug := range []string{"../../etc/passwd", "a/b", "..", "a b", "a%2fb"} {
		c, _ := start(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": 1, "slug": slug}},
				"meta": map[string]int{"page": 1, "pages": 1, "count": 1},
			})
		}))
		if _, err := c.List(context.Background(), SSP); !errors.Is(err, ErrBadID) {
			t.Errorf("%q: got %v, want ErrBadID", slug, err)
		}
	}
}

func TestExportAsksForValidatedOSCAL(t *testing.T) {
	f := newFake()
	f.add(SSP, "alpha", oscalBody(SSP, "a"))
	c, _ := start(t, f)
	res, err := c.Export(context.Background(), Document{Kind: SSP, Slug: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Body) != oscalBody(SSP, "a") || res.Unchanged {
		t.Fatalf("got %q unchanged=%v", res.Body, res.Unchanged)
	}
	if f.lastQ != "format=oscal&validate=true" {
		t.Errorf("query was %q", f.lastQ)
	}
}

func TestBasePathIsKeptAsAPrefix(t *testing.T) {
	f := newFake()
	f.add(SSP, "alpha", oscalBody(SSP, "a"))
	mux := http.NewServeMux()
	mux.Handle("/sparc/", http.StripPrefix("/sparc", f))
	c, _ := start(t, mux, func(cfg *Config) { cfg.BaseURL += "/sparc/" })
	if _, err := c.Export(context.Background(), Document{Kind: SSP, Slug: "alpha"}); err != nil {
		t.Fatal(err)
	}
}

// The acceptance measurement: a second pull with nothing changed transfers no
// bodies.
func TestUnchangedDocumentIsNotTransferredAgain(t *testing.T) {
	f := newFake()
	f.add(SAR, "alpha", oscalBody(SAR, "a"))
	cache, err := OpenCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := start(t, f, func(cfg *Config) { cfg.Cache = cache })
	d := Document{Kind: SAR, Slug: "alpha"}
	if _, err := c.Export(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	res, err := c.Export(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Unchanged || f.sent != 1 || f.notMod != 1 {
		t.Fatalf("unchanged=%v sent=%d 304s=%d, want true 1 1", res.Unchanged, f.sent, f.notMod)
	}
	if string(res.Body) != oscalBody(SAR, "a") {
		t.Errorf("cached body %q", res.Body)
	}

	// Changed upstream: a new body, a new ETag, transferred.
	f.bodies["sar_documents/alpha"] = oscalBody(SAR, "b")
	res, err = c.Export(context.Background(), d)
	if err != nil || res.Unchanged || f.sent != 2 {
		t.Fatalf("after a change: unchanged=%v sent=%d err=%v", res.Unchanged, f.sent, err)
	}
}

func TestATamperedCacheIsAMiss(t *testing.T) {
	f := newFake()
	f.add(SSP, "alpha", oscalBody(SSP, "a"))
	dir := t.TempDir()
	cache, _ := OpenCache(dir)
	c, _ := start(t, f, func(cfg *Config) { cfg.Cache = cache })
	d := Document{Kind: SSP, Slug: "alpha"}
	if _, err := c.Export(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ssp-alpha.json"), []byte(oscalBody(SSP, "forged")), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := c.Export(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if res.Unchanged || string(res.Body) != oscalBody(SSP, "a") || f.sent != 2 {
		t.Fatalf("a forged cache entry was served: %q unchanged=%v", res.Body, res.Unchanged)
	}
}

func TestCacheIsOwnerOnly(t *testing.T) {
	f := newFake()
	f.add(SSP, "alpha", oscalBody(SSP, "a"))
	dir := filepath.Join(t.TempDir(), "c")
	cache, _ := OpenCache(dir)
	c, _ := start(t, f, func(cfg *Config) { cfg.Cache = cache })
	if _, err := c.Export(context.Background(), Document{Kind: SSP, Slug: "alpha"}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{dir, filepath.Join(dir, "ssp-alpha.json"), filepath.Join(dir, "ssp-alpha.etag.json")} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s is %v, want owner-only", p, st.Mode().Perm())
		}
	}
}

func TestRefusalsAreNamed(t *testing.T) {
	f := newFake()
	f.add(SSP, "broken", oscalBody(SSP, "x"))
	f.status["ssp_documents/broken"] = http.StatusUnprocessableEntity
	f.add(SSP, "fields", `{"document_name":"x","controls":[]}`)
	f.add(SSP, "wrongmodel", oscalBody(SAR, "x"))
	f.add(SSP, "twoRoots", `{"system-security-plan":{},"extra":{}}`)
	f.add(SSP, "huge", oscalBody(SSP, strings.Repeat("x", 2048)))
	c, _ := start(t, f, func(cfg *Config) { cfg.MaxBytes = 1024 })

	cases := map[string]error{
		"broken":     ErrNotConformed,
		"fields":     ErrNotOSCAL,
		"wrongmodel": ErrNotOSCAL,
		"twoRoots":   ErrNotOSCAL,
		"huge":       ErrTooLarge,
		"missing":    ErrNotFound,
	}
	for slug, want := range cases {
		_, err := c.Export(context.Background(), Document{Kind: SSP, Slug: slug})
		if !errors.Is(err, want) {
			t.Errorf("%s: got %v, want %v", slug, err, want)
		}
	}
	_, err := c.Export(context.Background(), Document{Kind: SSP, Slug: "broken"})
	if !strings.Contains(err.Error(), "missing title") {
		t.Errorf("SPARC's detail was lost: %v", err)
	}
}

func TestARefusedTokenIsNamedAndNeverEchoed(t *testing.T) {
	c, _ := start(t, newFake(), func(cfg *Config) { cfg.Token = "sparc_wrongwrongwrong" })
	_, err := c.List(context.Background(), SSP)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("got %v, want ErrUnauthorized", err)
	}
	if strings.Contains(err.Error(), "wrongwrong") {
		t.Fatalf("the token is in the error: %v", err)
	}
}

func TestARedirectOffHostIsRefused(t *testing.T) {
	other := httptest.NewTLSServer(newFake())
	t.Cleanup(other.Close)
	var seen string
	c, _ := start(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/api/v1/ssp_documents", http.StatusFound)
	}))
	other.Config.Handler = http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
	})
	_, err := c.List(context.Background(), SSP)
	if !errors.Is(err, ErrRedirect) {
		t.Fatalf("got %v, want ErrRedirect", err)
	}
	if seen != "" {
		t.Fatalf("the other host received Authorization %q", seen)
	}
}

func TestARedirectOnTheSameHostIsFollowed(t *testing.T) {
	f := newFake()
	f.add(SSP, "alpha", oscalBody(SSP, "a"))
	mux := http.NewServeMux()
	mux.HandleFunc("/old/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/v1/ssp_documents/alpha/export?format=oscal&validate=true", http.StatusMovedPermanently)
	})
	mux.Handle("/api/", f)
	c, _ := start(t, mux, func(cfg *Config) { cfg.BaseURL += "/old" })
	if _, err := c.Export(context.Background(), Document{Kind: SSP, Slug: "alpha"}); err != nil {
		t.Fatal(err)
	}
}

func TestUnexpectedStatusIsAnError(t *testing.T) {
	c, _ := start(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream down"))
	}))
	_, err := c.List(context.Background(), SSP)
	if !errors.Is(err, ErrStatus) || !strings.Contains(err.Error(), "upstream down") {
		t.Fatalf("got %v", err)
	}
}

func TestA304WithoutACacheIsAnError(t *testing.T) {
	c, _ := start(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	if _, err := c.Export(context.Background(), Document{Kind: SSP, Slug: "a"}); !errors.Is(err, ErrStatus) {
		t.Fatalf("got %v", err)
	}
}

func TestDocumentKeyFallsBackToTheID(t *testing.T) {
	if k := (Document{ID: "7"}).Key(); k != "7" {
		t.Fatalf("got %q", k)
	}
	if k := (Document{ID: "7", Slug: "s"}).Key(); k != "s" {
		t.Fatalf("got %q", k)
	}
}
