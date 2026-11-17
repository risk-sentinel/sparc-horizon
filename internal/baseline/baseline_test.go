package baseline

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

var today = time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC)

const accepted = `findings:
  - cve_id: CVE-2026-11111
    image: risksentinel/sparc-horizon
    image_tag_first_seen: v0.1.0
    package: tzdata
    installed_version: 2026f-1.el10_3
    fixed_version: null
    severity: HIGH
    disposition: accepted
    rationale: "Data package; nothing parses the affected file."
    nist_control: si-2
    reviewed_by: "@owner"
    discovery_date: "2026-11-16"
    last_reviewed: "2026-11-16"
    next_review_date: "2027-02-16"
`

func parse(t *testing.T, doc string) *Baseline {
	t.Helper()
	b, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return b
}

func TestTheCommittedBaselineIsValid(t *testing.T) {
	f, err := os.Open("../../container-baseline.yml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()
	b, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if errs := b.Validate(time.Now()); len(errs) > 0 {
		t.Fatalf("container-baseline.yml is invalid: %v", errs)
	}
}

func TestAnAcceptedFindingIsIgnoredUntilItsReviewDate(t *testing.T) {
	b := parse(t, accepted)
	if errs := b.Validate(today); len(errs) > 0 {
		t.Fatalf("valid entry rejected: %v", errs)
	}
	if got := b.IgnoreIDs(); len(got) != 1 || got[0] != "CVE-2026-11111" {
		t.Fatalf("IgnoreIDs = %v, want [CVE-2026-11111]", got)
	}

	// The day after the review date the acceptance has lapsed, and that is an
	// error rather than a silent un-ignore.
	errs := b.Validate(time.Date(2027, 2, 17, 0, 0, 0, 0, time.UTC))
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "has passed") {
		t.Fatalf("expired acceptance: errs = %v, want one 'has passed'", errs)
	}
	// On the review date itself it still holds.
	if errs := b.Validate(time.Date(2027, 2, 16, 23, 0, 0, 0, time.UTC)); len(errs) > 0 {
		t.Fatalf("review date itself rejected: %v", errs)
	}
}

func TestRemediatedFindingsNeverSuppress(t *testing.T) {
	doc := strings.NewReplacer(
		"disposition: accepted", "disposition: remediated",
		`next_review_date: "2027-02-16"`, `remediated_date: "2026-11-16"`,
	).Replace(accepted)
	b := parse(t, doc)
	if errs := b.Validate(today); len(errs) > 0 {
		t.Fatalf("valid remediated entry rejected: %v", errs)
	}
	if got := b.IgnoreIDs(); len(got) != 0 {
		t.Fatalf("a remediated finding is suppressed: %v", got)
	}
}

func TestEveryProblemIsReported(t *testing.T) {
	cases := map[string]struct {
		from, to string
		want     string
	}{
		"bad id":              {"CVE-2026-11111", "CVE-26-1", "not a CVE or GHSA"},
		"no rationale":        {`rationale: "Data package; nothing parses the affected file."`, `rationale: ""`, "rationale is required"},
		"medium":              {"severity: HIGH", "severity: MEDIUM", "CRITICAL and HIGH only"},
		"control case":        {"nist_control: si-2", "nist_control: SI-2", "nist_control"},
		"unknown disposition": {"disposition: accepted", "disposition: ignored", "want accepted, deferred or remediated"},
		"no review date":      {`    next_review_date: "2027-02-16"` + "\n", "", "must carry one"},
		"bad date":            {`discovery_date: "2026-11-16"`, `discovery_date: "16/11/2026"`, "not YYYY-MM-DD"},
		"remediated with review": {
			"disposition: accepted", "disposition: remediated", "remediation is terminal",
		},
		"accepted with remediated date": {
			`last_reviewed: "2026-11-16"`, `last_reviewed: "2026-11-16"` + "\n" + `    remediated_date: "2026-11-16"`,
			"remediated_date is set",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			doc := strings.Replace(accepted, c.from, c.to, 1)
			if doc == accepted {
				t.Fatalf("replacement %q did not apply", c.from)
			}
			errs := parse(t, doc).Validate(today)
			found := false
			for _, e := range errs {
				if strings.Contains(e.Error(), c.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("errs = %v, want one containing %q", errs, c.want)
			}
		})
	}
}

func TestDuplicatesAreRejected(t *testing.T) {
	doc := accepted + strings.TrimPrefix(accepted, "findings:\n")
	errs := parse(t, doc).Validate(today)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "duplicate") {
		t.Fatalf("errs = %v, want one duplicate", errs)
	}
}

func TestParseRefusesWhatWouldDropADecision(t *testing.T) {
	for name, doc := range map[string]string{
		"empty file":    "",
		"no findings":   "other: 1\n",
		"unknown key":   strings.Replace(accepted, "rationale:", "justification:", 1),
		"not a mapping": "- 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(doc)); err == nil {
				t.Fatal("Parse accepted it")
			}
		})
	}
	if _, err := Parse(strings.NewReader("findings: []\n")); err != nil {
		t.Fatalf("an explicitly empty baseline is valid: %v", err)
	}
}

func TestTrivyIgnore(t *testing.T) {
	var buf bytes.Buffer
	if err := parse(t, accepted).WriteTrivyIgnore(&buf, "container-baseline.yml"); err != nil {
		t.Fatalf("WriteTrivyIgnore: %v", err)
	}
	want := "# Generated from container-baseline.yml: 1 accepted or deferred finding(s). Do not edit.\nCVE-2026-11111\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}

	buf.Reset()
	if err := parse(t, "findings: []\n").WriteTrivyIgnore(&buf, "x"); err != nil {
		t.Fatalf("WriteTrivyIgnore: %v", err)
	}
	if strings.Count(buf.String(), "\n") != 1 || !strings.HasPrefix(buf.String(), "#") {
		t.Fatalf("empty baseline should write only the header, got %q", buf.String())
	}
}
