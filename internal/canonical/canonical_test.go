package canonical

import (
	"errors"
	"strings"
	"testing"
)

// The defect that motivated the separator change (#30). Under "|" the two
// tuples produce identical joined input and therefore identical UUIDs; under
// the unit separator they are distinguishable, and a field carrying the
// separator is rejected rather than escaped.
func TestSeparatorRemovesJoinAmbiguity(t *testing.T) {
	const old = "|"
	if strings.Join([]string{"a|b", "c"}, old) != strings.Join([]string{"a", "b|c"}, old) {
		t.Fatal("premise wrong: the two tuples should collide under the old delimiter")
	}

	x, err := Join("a|b", "c")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	y, err := Join("a", "b|c")
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if x == y {
		t.Errorf("tuples still collide under the unit separator: %q", x)
	}
}

func TestJoinRejectsSeparatorRatherThanEscapingIt(t *testing.T) {
	if _, err := Join("ok", "bad"+Separator+"field"); !errors.Is(err, ErrSeparatorInField) {
		t.Errorf("want ErrSeparatorInField, got %v", err)
	}
	if err := ValidateField("clean"); err != nil {
		t.Errorf("clean field rejected: %v", err)
	}
}

func TestJoinOrderIsSignificant(t *testing.T) {
	a, _ := Join("x", "y")
	b, _ := Join("y", "x")
	if a == b {
		t.Error("field order must change the joined input")
	}
}

func TestPeriod(t *testing.T) {
	// A quarter or a month, and nothing else. Narrowed to SPARC's shared
	// contract (#54): a bare year and a full date were accepted here first,
	// and two implementations disagreeing about what a period is derive two
	// identifiers for one object. A date belongs to a decision — see
	// TestDecisionDate.
	valid := []string{"2026-Q1", "2026-Q4", "2026-01", "2026-12"}
	for _, v := range valid {
		if got, err := Period(v); err != nil || got != v {
			t.Errorf("Period(%q) = %q, %v; want it accepted unchanged", v, got, err)
		}
	}
	invalid := []string{"2026Q3", "Q3-2026", "2026-1", "2026-1-5", "2026-13", "2026-Q0", "2026-Q5",
		"2026-00", "2026-01-32", "26-01", "", "2026-01-", "2026-W01",
		"2026", "2026-09-20", "2026-02-29"}
	for _, v := range invalid {
		if _, err := Period(v); err == nil {
			t.Errorf("Period(%q) accepted; want rejected", v)
		}
	}
}

func TestUUIDLowercasesAndRejectsMalformed(t *testing.T) {
	const upper = "3FA85F64-5717-4562-B3FC-2C963F66AFA6"
	const lower = "3fa85f64-5717-4562-b3fc-2c963f66afa6"
	got, err := UUID(upper)
	if err != nil {
		t.Fatalf("UUID: %v", err)
	}
	if got != lower {
		t.Errorf("UUID(%q) = %q; want %q", upper, got, lower)
	}
	if a, _ := UUID(lower); a != got {
		t.Error("the two spellings must canonicalise to one value")
	}
	for _, v := range []string{"not-a-uuid", "", "3fa85f64-5717-4562-b3fc", lower + "-extra",
		"3fa85f64_5717_4562_b3fc_2c963f66afa6", "3fa85f64-5717-4562-b3fc-2c963f66afag"} {
		if _, err := UUID(v); err == nil {
			t.Errorf("UUID(%q) accepted; want rejected", v)
		}
	}
}

// NFC only. Case is preserved: a field needing case folding to match is the
// wrong field, so folding is not this package's job.
func TestStringNormalisesToNFCWithoutFoldingOrTrimming(t *testing.T) {
	const decomposed = "é" // e + combining acute
	const precomposed = "é" // é
	if String(decomposed) != precomposed {
		t.Error("NFC did not compose the decomposed form")
	}
	if String(precomposed) != precomposed {
		t.Error("NFC is not idempotent on an already-composed value")
	}
	if String("  padded  ") != "  padded  " {
		t.Error("String must not trim")
	}
	if String("MixedCase") != "MixedCase" {
		t.Error("String must not case-fold")
	}
}

// A decision is keyed on the day it was taken, so that the value lines up with
// the next-decision-date prop the HUD counts down to. A quarter cannot be
// matched against a calendar.
func TestDecisionDate(t *testing.T) {
	for _, v := range []string{"2026-10-11", "2026-01-01", "2026-12-31"} {
		if got, err := DecisionDate(v); err != nil || got != v {
			t.Errorf("DecisionDate(%q) = %q, %v; want it accepted unchanged", v, got, err)
		}
	}
	for _, v := range []string{"2026-Q4", "2026-10", "2026", "11-10-2026", "", "2026-1-1"} {
		if _, err := DecisionDate(v); err == nil {
			t.Errorf("DecisionDate(%q) accepted; want rejected", v)
		}
	}

	// Calendar-impossible dates pass, because the shared contract types this
	// as ^\d{4}-\d{2}-\d{2}$ and nothing more. Rejecting them here would
	// mean refusing to ingest an object a peer has already derived an
	// identifier for, which is the failure the shared grammar exists to
	// prevent. Raised upstream rather than fixed unilaterally.
	for _, v := range []string{"2026-13-01", "2026-02-31"} {
		if _, err := DecisionDate(v); err != nil {
			t.Errorf("DecisionDate(%q) rejected; the shared contract accepts it", v)
		}
	}
}
