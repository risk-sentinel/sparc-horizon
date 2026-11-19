// Package sparc pulls OSCAL documents from a SPARC instance's /api/v1.
//
// What SPARC serves was read from its source, not assumed (#124, sparc
// origin/main 44585a6d):
//
//   - Authentication is `Authorization: Bearer`, a `sparc_` service token or an
//     OIDC JWT. There is no client certificate.
//   - A list is `{data: [...], meta: {page, pages, count, items}}`.
//   - Each document is at `/api/v1/<kind>_documents/<id>/export?format=oscal`.
//     SPARC validates the export against the NIST schema by default and answers
//     422 for a document that does not conform. This client never asks for the
//     unvalidated escape hatch.
//   - The export's strong ETag digests the exported bytes, so a 304 means the
//     bytes are unchanged, not merely that `updated_at` is.
//
// Transport rules are owner decisions (2026-11-19), not defaults: https only,
// on every host including loopback; TLS 1.2 or later; the system's trust roots,
// with no per-deployment CA file. Plain http is not FedRAMP-compliant for a
// High system, so no flag, variable or option in the shipped code relaxes it.
//
// This is a fetch, not ingestion. Nothing here verifies a bundle signature, so
// nothing here is TM-1's ingestion path (P2).
package sparc

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Kind is one of the document types SPARC exports as OSCAL.
type Kind string

// The kinds Horizon reads, in the evidence chain's order. The assessment plan
// is needed because assessment results reach their SSP through it (import-ap).
const (
	SSP  Kind = "ssp"
	SAP  Kind = "sap"
	SAR  Kind = "sar"
	POAM Kind = "poam"
)

// Kinds is every kind, in pull order.
var Kinds = []Kind{SSP, SAP, SAR, POAM}

// root is the OSCAL model key a kind's export must carry. A body without it is
// not the OSCAL document asked for: SPARC's own field JSON, an error page, or a
// different model.
func (k Kind) root() string {
	switch k {
	case SSP:
		return "system-security-plan"
	case SAP:
		return "assessment-plan"
	case SAR:
		return "assessment-results"
	case POAM:
		return "plan-of-action-and-milestones"
	}
	return ""
}

func (k Kind) collection() string { return string(k) + "_documents" }

// Limits. A list longer than maxPages pages, or a document larger than the
// cap, is refused rather than read without bound.
const (
	DefaultMaxBytes = 64 << 20 // the mock's own per-file cap (view.MaxDocumentBytes)
	DefaultTimeout  = 60 * time.Second
	pageSize        = 100
	maxPages        = 1000
	maxErrorBody    = 4 << 10
)

// Errors a caller can tell apart. Each wraps the detail SPARC gave, which
// never includes the token.
var (
	ErrInsecureURL  = errors.New("sparc: the URL must be https")
	ErrBadURL       = errors.New("sparc: the URL is not usable")
	ErrNoToken      = errors.New("sparc: no API token")
	ErrUnauthorized = errors.New("sparc: the token was refused")
	ErrNotFound     = errors.New("sparc: not found")
	ErrNotConformed = errors.New("sparc: SPARC refused to export a document that does not conform to the OSCAL schema")
	ErrTooLarge     = errors.New("sparc: response exceeds the size cap")
	ErrRedirect     = errors.New("sparc: redirect refused")
	ErrNotOSCAL     = errors.New("sparc: the response is not the OSCAL document requested")
	ErrList         = errors.New("sparc: the document list is inconsistent")
	ErrStatus       = errors.New("sparc: unexpected response")
	ErrBadID        = errors.New("sparc: unusable document id")
)

// Config configures a Client.
type Config struct {
	// BaseURL is the SPARC instance, e.g. https://sparc.example.gov. A path is
	// kept as a prefix; a query, fragment or userinfo is refused.
	BaseURL string
	// Token is the Bearer token. It is sent only to BaseURL's host.
	Token string
	// Cache, when set, holds ETags and bodies so an unchanged document is not
	// transferred again.
	Cache *Cache
	// Timeout bounds one request. Zero means DefaultTimeout.
	Timeout time.Duration
	// MaxBytes caps one response body. Zero means DefaultMaxBytes.
	MaxBytes int64

	// roots replaces the system trust roots. Unexported on purpose: only this
	// package's tests can set it, to trust an httptest TLS server. The shipped
	// binary has no way to load a CA of its own choosing (owner, 2026-11-19).
	roots *x509.CertPool
}

// Client talks to one SPARC instance.
type Client struct {
	base     *url.URL
	token    string
	http     *http.Client
	cache    *Cache
	maxBytes int64
}

// New validates cfg and returns a Client. Nothing is sent until a call.
func New(cfg Config) (*Client, error) {
	base, err := parseBase(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	if err := checkToken(cfg.Token); err != nil {
		return nil, err
	}
	c := &Client{base: base, token: cfg.Token, cache: cfg.Cache, maxBytes: cfg.MaxBytes}
	if c.maxBytes <= 0 {
		c.maxBytes = DefaultMaxBytes
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	c.http = &http.Client{
		Timeout:       timeout,
		Transport:     transport(cfg.roots),
		CheckRedirect: c.checkRedirect,
	}
	return c, nil
}

// NIST: SC-8, SC-13 — every request is TLS 1.2 or later, verified against the
// system trust roots; there is no plain-http path and no verification bypass.
func transport(roots *x509.CertPool) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	return t
}

// NIST: SC-8 — the scheme is checked here, once, for every host. Loopback is
// not an exception (owner, 2026-11-19).
func parseBase(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadURL, err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("%w: got %q", ErrInsecureURL, u.Scheme+"://")
	}
	switch {
	case u.Host == "":
		return nil, fmt.Errorf("%w: no host", ErrBadURL)
	case u.User != nil:
		return nil, fmt.Errorf("%w: credentials in the URL; use the token file", ErrBadURL)
	case u.RawQuery != "" || u.Fragment != "":
		return nil, fmt.Errorf("%w: a query or fragment is not a base URL", ErrBadURL)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

// NIST: IA-5 — a token that is empty or carries whitespace or control
// characters is refused before it can reach a header.
func checkToken(tok string) error {
	if tok == "" {
		return ErrNoToken
	}
	for _, r := range tok {
		if r <= ' ' || r == 0x7f {
			return fmt.Errorf("%w: the token contains whitespace or a control character", ErrNoToken)
		}
	}
	return nil
}

// checkRedirect follows a redirect only to the same scheme and host. Go
// already drops Authorization on a cross-host redirect; refusing outright
// means the caller learns SPARC sent one instead of reading someone else's
// response as SPARC's.
func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("%w: more than 5", ErrRedirect)
	}
	if req.URL.Scheme != c.base.Scheme || req.URL.Host != c.base.Host {
		return fmt.Errorf("%w: to %s://%s", ErrRedirect, req.URL.Scheme, req.URL.Host)
	}
	return nil
}

// Document is one row of a SPARC document list.
type Document struct {
	Kind      Kind
	ID        string // SPARC's numeric id, as text
	Slug      string
	UUID      string
	Name      string
	UpdatedAt string
}

// Key is the identifier the export is requested by and cached under: the
// slug when SPARC has one, otherwise the id.
func (d Document) Key() string {
	if d.Slug != "" {
		return d.Slug
	}
	return d.ID
}

type listRow struct {
	ID        json.Number `json:"id"`
	Slug      string      `json:"slug"`
	UUID      string      `json:"uuid"`
	Name      string      `json:"name"`
	UpdatedAt string      `json:"updated_at"`
}

type listPage struct {
	Data []listRow `json:"data"`
	Meta *struct {
		Page  int `json:"page"`
		Pages int `json:"pages"`
		Count int `json:"count"`
	} `json:"meta"`
}

// List returns every document of a kind the token can read, across pages.
// A page that disagrees with the first one about the total is an error: the
// list changed during the walk, and a partial list would silently drop a
// document.
func (c *Client) List(ctx context.Context, k Kind) ([]Document, error) {
	if k.root() == "" {
		return nil, fmt.Errorf("%w: unknown kind %q", ErrList, k)
	}
	var out []Document
	count, pages := -1, 1
	for page := 1; page <= pages; page++ {
		p, err := c.listPage(ctx, k, page)
		if err != nil {
			return nil, err
		}
		if count >= 0 && (p.Meta.Count != count || p.Meta.Pages != pages) {
			return nil, fmt.Errorf("%w: %s changed while it was being read", ErrList, k.collection())
		}
		count, pages = p.Meta.Count, p.Meta.Pages
		rows, err := documents(k, p.Data)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	if len(out) != max(count, 0) {
		return nil, fmt.Errorf("%w: %s reported %d documents and listed %d", ErrList, k.collection(), count, len(out))
	}
	return out, nil
}

// listPage fetches and decodes one page, refusing one whose meta is absent
// or names a different page.
func (c *Client) listPage(ctx context.Context, k Kind, page int) (listPage, error) {
	var p listPage
	if page > maxPages {
		return p, fmt.Errorf("%w: more than %d pages of %s", ErrList, maxPages, k.collection())
	}
	q := url.Values{"page": {strconv.Itoa(page)}, "items": {strconv.Itoa(pageSize)}}
	body, _, err := c.get(ctx, c.endpoint(q, k.collection()), "")
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return p, fmt.Errorf("%w: %s page %d is not a list: %w", ErrList, k.collection(), page, err)
	}
	if p.Meta == nil || p.Meta.Page != page {
		return p, fmt.Errorf("%w: %s page %d has no matching meta", ErrList, k.collection(), page)
	}
	return p, nil
}

// documents converts list rows, refusing any whose key is unusable.
func documents(k Kind, rows []listRow) ([]Document, error) {
	out := make([]Document, 0, len(rows))
	for _, r := range rows {
		d := Document{Kind: k, ID: r.ID.String(), Slug: r.Slug, UUID: r.UUID, Name: r.Name, UpdatedAt: r.UpdatedAt}
		if err := checkKey(d.Key()); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// Result is one export.
type Result struct {
	Body []byte
	// Unchanged is true when SPARC answered 304 and Body came from the cache.
	Unchanged bool
}

// Export fetches a document's validated OSCAL JSON. With a cache it sends the
// stored ETag, and a 304 is answered from the cached bytes after their digest
// is checked.
func (c *Client) Export(ctx context.Context, d Document) (Result, error) {
	key := d.Key()
	if err := checkKey(key); err != nil {
		return Result{}, err
	}
	q := url.Values{"format": {"oscal"}, "validate": {"true"}}
	u := c.endpoint(q, d.Kind.collection(), key, "export")

	var cached *entry
	if c.cache != nil {
		if e, ok := c.cache.load(d.Kind, key); ok {
			cached = e
		}
	}
	etag := ""
	if cached != nil {
		etag = cached.ETag
	}
	body, newTag, err := c.get(ctx, u, etag)
	if errors.Is(err, errNotModified) {
		if cached == nil {
			return Result{}, fmt.Errorf("%w: 304 without a cached copy", ErrStatus)
		}
		return Result{Body: cached.Body, Unchanged: true}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("%s %s: %w", d.Kind, key, err)
	}
	if err := checkModel(body, d.Kind); err != nil {
		return Result{}, fmt.Errorf("%s %s: %w", d.Kind, key, err)
	}
	if c.cache != nil && newTag != "" {
		if err := c.cache.store(d.Kind, key, newTag, body); err != nil {
			return Result{}, err
		}
	}
	return Result{Body: body}, nil
}

// checkModel confirms the body is one JSON object whose only top-level key is
// the kind's OSCAL model root.
func checkModel(body []byte, k Kind) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return fmt.Errorf("%w: not a JSON object", ErrNotOSCAL)
	}
	if _, ok := top[k.root()]; !ok || len(top) != 1 {
		keys := make([]string, 0, len(top))
		for name := range top {
			keys = append(keys, name)
		}
		return fmt.Errorf("%w: want only %q at the top, got %q", ErrNotOSCAL, k.root(), keys)
	}
	return nil
}

// checkKey admits the characters a SPARC slug or id is made of. It is the
// guard against a list row naming a path: the key becomes a URL segment and a
// cache file name.
func checkKey(key string) error {
	if key == "" || len(key) > 200 || key == "." || key == ".." {
		return fmt.Errorf("%w: %q", ErrBadID, key)
	}
	for _, r := range key {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
		if !ok {
			return fmt.Errorf("%w: %q", ErrBadID, key)
		}
	}
	return nil
}

func (c *Client) endpoint(q url.Values, segments ...string) string {
	u := *c.base
	parts := append([]string{u.Path, "api", "v1"}, segments...)
	u.Path = strings.Join(parts, "/")
	u.RawPath = ""
	u.RawQuery = q.Encode()
	return u.String()
}

var errNotModified = errors.New("304")

// get performs one GET and returns the body and ETag of a 200.
func (c *Client) get(ctx context.Context, u, ifNoneMatch string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrBadURL, err)
	}
	// NIST: IA-2, IA-5 — the service authenticates to SPARC as itself with a
	// Bearer token. The token is never logged and never part of an error.
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", scrub(err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := readCapped(resp.Body, c.maxBytes)
		if err != nil {
			return nil, "", err
		}
		return body, resp.Header.Get("ETag"), nil
	case http.StatusNotModified:
		return nil, "", errNotModified
	}
	detail := sparcError(resp.Body)
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, "", fmt.Errorf("%w (%d): %s", ErrUnauthorized, resp.StatusCode, detail)
	case http.StatusNotFound:
		return nil, "", fmt.Errorf("%w: %s", ErrNotFound, detail)
	case http.StatusUnprocessableEntity:
		return nil, "", fmt.Errorf("%w: %s", ErrNotConformed, detail)
	}
	return nil, "", fmt.Errorf("%w: HTTP %d: %s", ErrStatus, resp.StatusCode, detail)
}

// scrub returns a transport error with the request URL kept and nothing else
// added. A url.Error carries the URL, never headers, so the token cannot be in
// it; the wrapping keeps the redirect sentinel visible to errors.Is.
func scrub(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("sparc: GET %s: %w", ue.URL, ue.Err)
	}
	return err
}

func readCapped(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("sparc: reading the response: %w", err)
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%w: over %d bytes", ErrTooLarge, limit)
	}
	return b, nil
}

// sparcError extracts SPARC's `{error, details}` body, bounded, as one line.
func sparcError(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, maxErrorBody))
	var e struct {
		Error   string   `json:"error"`
		Details []string `json:"details"`
	}
	if json.Unmarshal(b, &e) == nil && e.Error != "" {
		if len(e.Details) > 0 {
			return e.Error + ": " + strings.Join(e.Details, "; ")
		}
		return e.Error
	}
	return strings.TrimSpace(string(bytes.ToValidUTF8(b, nil)))
}
