// Package baseline reads container-baseline.yml, the CRITICAL/HIGH CVE
// dispositions for the image this repository publishes (S1-10), and derives
// the Trivy ignore list the release gate uses.
//
// The file is the record of a risk decision, so a malformed or stale entry is
// an error rather than something to skip: an accepted finding past its review
// date stops suppressing anything and fails validation, which is how an
// acceptance is forced back in front of the owner instead of lapsing silently
// (RA-5, SI-2, CA-5).
package baseline

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Disposition values, as container-build-sign and sparc-iac use them.
const (
	Accepted   = "accepted"
	Deferred   = "deferred"
	Remediated = "remediated"
)

const dateLayout = "2006-01-02"

var (
	idRe       = regexp.MustCompile(`^(CVE-\d{4}-\d{4,}|GHSA(-[23456789cfghjmpqrvwx]{4}){3})$`)
	controlRe  = regexp.MustCompile(`^[a-z]{2}-\d{1,2}(\(\d{1,2}\))?$`)
	severities = map[string]bool{"CRITICAL": true, "HIGH": true}
)

// Finding is one disposition entry.
type Finding struct {
	CVEID             string  `yaml:"cve_id"`
	Image             string  `yaml:"image"`
	ImageTagFirstSeen string  `yaml:"image_tag_first_seen"`
	Package           string  `yaml:"package"`
	InstalledVersion  string  `yaml:"installed_version"`
	FixedVersion      *string `yaml:"fixed_version"`
	Severity          string  `yaml:"severity"`
	Disposition       string  `yaml:"disposition"`
	Rationale         string  `yaml:"rationale"`
	NISTControl       string  `yaml:"nist_control"`
	ReviewedBy        string  `yaml:"reviewed_by"`
	DiscoveryDate     string  `yaml:"discovery_date"`
	LastReviewed      string  `yaml:"last_reviewed"`
	NextReviewDate    string  `yaml:"next_review_date"`
	RemediatedDate    string  `yaml:"remediated_date"`
}

// Baseline is the whole file.
type Baseline struct {
	Findings []Finding `yaml:"findings"`
}

// Parse decodes a baseline, refusing unknown fields so a misspelt key cannot
// quietly drop a required value.
func Parse(r io.Reader) (*Baseline, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	var b Baseline
	if err := dec.Decode(&b); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("baseline is empty: want a document with a findings list")
		}
		return nil, fmt.Errorf("decode baseline: %w", err)
	}
	if b.Findings == nil {
		return nil, errors.New("baseline has no findings key: write `findings: []` for none")
	}
	return &b, nil
}

// Validate returns every problem in the baseline as of the given day, not just
// the first, so one run shows the whole repair list.
func (b *Baseline) Validate(today time.Time) []error {
	var errs []error
	seen := map[string]bool{}
	day := truncate(today)
	for i, f := range b.Findings {
		where := fmt.Sprintf("findings[%d] %s", i, f.CVEID)
		key := f.CVEID + "|" + f.Image
		if seen[key] {
			errs = append(errs, fmt.Errorf("%s: duplicate entry for image %q", where, f.Image))
		}
		seen[key] = true
		errs = append(errs, f.validate(where, day)...)
	}
	return errs
}

func (f Finding) validate(where string, day time.Time) []error {
	var errs []error
	bad := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf("%s: "+format, append([]any{where}, args...)...))
	}

	if !idRe.MatchString(f.CVEID) {
		bad("cve_id %q is not a CVE or GHSA identifier", f.CVEID)
	}
	required := map[string]string{
		"image": f.Image, "image_tag_first_seen": f.ImageTagFirstSeen, "package": f.Package,
		"installed_version": f.InstalledVersion, "rationale": f.Rationale, "reviewed_by": f.ReviewedBy,
	}
	for _, k := range sortedKeys(required) {
		if strings.TrimSpace(required[k]) == "" {
			bad("%s is required", k)
		}
	}
	if !severities[f.Severity] {
		bad("severity %q: the baseline records CRITICAL and HIGH only", f.Severity)
	}
	if !controlRe.MatchString(f.NISTControl) {
		bad("nist_control %q is not a lower-case 800-53 control such as si-2", f.NISTControl)
	}
	for _, d := range []struct{ name, value string }{
		{"discovery_date", f.DiscoveryDate}, {"last_reviewed", f.LastReviewed},
	} {
		if _, err := parseDate(d.value); err != nil {
			bad("%s: %v", d.name, err)
		}
	}

	switch f.Disposition {
	case Accepted, Deferred:
		next, err := parseDate(f.NextReviewDate)
		switch {
		case err != nil:
			bad("next_review_date: %v (an %s finding must carry one)", err, f.Disposition)
		case next.Before(day):
			bad("next_review_date %s has passed: re-review the finding or remediate it", f.NextReviewDate)
		}
		if f.RemediatedDate != "" {
			bad("remediated_date is set on a %s finding", f.Disposition)
		}
	case Remediated:
		if _, err := parseDate(f.RemediatedDate); err != nil {
			bad("remediated_date: %v (a remediated finding must carry one)", err)
		}
		if f.NextReviewDate != "" {
			bad("next_review_date is set on a remediated finding: remediation is terminal")
		}
	default:
		bad("disposition %q: want accepted, deferred or remediated", f.Disposition)
	}
	return errs
}

// IgnoreIDs is what the release gate may skip: accepted and deferred findings,
// sorted and de-duplicated. Remediated findings never suppress anything, so a
// regression of a fixed CVE fails the gate.
func (b *Baseline) IgnoreIDs() []string {
	set := map[string]bool{}
	for _, f := range b.Findings {
		if f.Disposition == Accepted || f.Disposition == Deferred {
			set[f.CVEID] = true
		}
	}
	return sortedKeys(set)
}

// WriteTrivyIgnore writes the ignore file Trivy reads, one identifier per line,
// with a header naming where it came from. An empty baseline writes only the
// header, which ignores nothing.
func (b *Baseline) WriteTrivyIgnore(w io.Writer, source string) error {
	ids := b.IgnoreIDs()
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Generated from %s: %d accepted or deferred finding(s). Do not edit.\n", source, len(ids))
	for _, id := range ids {
		sb.WriteString(id)
		sb.WriteByte('\n')
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("missing")
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q is not YYYY-MM-DD", s)
	}
	return t, nil
}

func truncate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
