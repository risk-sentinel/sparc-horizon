package canonical

import "testing"

// The equivalence that #36 requires and sparc#1162 explains: SPARC's own API
// emits control identifiers in more than one spelling, so a deriver that
// assumes the canonical form arrives canonical mints two identifiers for one
// control.
func TestNISTSpellingsConverge(t *testing.T) {
	groups := map[string][]string{
		"ac-2":   {"AC-2", "ac-2", "AC-02", "aC-2"},
		"ac-2.1": {"AC-2(1)", "AC-2 (1)", "AC-2.1", "ac-2.1", "AC-02(01)"},
		"si-4":   {"SI-4", "si-04"},
		// Three-letter prefixes and nested enhancements, both admitted by
		// SPARC's shared contract (#54). Being stricter than the shared
		// grammar means rejecting keys a peer legitimately derives.
		"sar-5":    {"SAR-5", "sar-05"},
		"ac-2.1.3": {"ac-2.1.3", "AC-2(1)(3)", "AC-2.1(3)"},
	}
	for want, spellings := range groups {
		for _, in := range spellings {
			got, err := ControlID(VocabNIST80053, in)
			if err != nil {
				t.Errorf("ControlID(%q): %v", in, err)
				continue
			}
			if got != want {
				t.Errorf("ControlID(%q) = %q, want %q", in, got, want)
			}
		}
	}
}

func TestNISTRejectsWhatIsNotAControlID(t *testing.T) {
	for _, in := range []string{
		"ac-2_smt.a", // a statement, not a control
		"AC2",        // the hyphen is not optional
		"ACM.1",      // an AWS Security Hub id, under the wrong vocabulary
		"a-2",
		"abcd-2", // the contract allows two or three letters, not four
		"ac-0",   // no control is numbered zero
		"ac-2.0", // nor any enhancement
		"ac-",
		"ac-2(1",
		"ac-2)1(",
		"",
		"  ac-2  ", // no trimming anywhere in this package
	} {
		if got, err := ControlID(VocabNIST80053, in); err == nil {
			t.Errorf("ControlID(%q) = %q, want an error", in, got)
		}
	}
}

// The reason canonicalisation is scoped at all. "ACM.1" lowercased is a
// well-formed NIST control id that names nothing in any catalog.
func TestOpaqueVocabularyIsCarriedAsIssued(t *testing.T) {
	for _, in := range []string{"ACM.1", "IAM.4", "1.1.1", "S3.5", "CIS-2.1.1"} {
		got, err := ControlID(VocabOpaque, in)
		if err != nil {
			t.Errorf("ControlID(%q): %v", in, err)
			continue
		}
		if got != in {
			t.Errorf("ControlID(%q) = %q; an opaque identifier must be carried unchanged", in, got)
		}
	}
}

func TestVocabularyIsRequired(t *testing.T) {
	if _, err := ControlID(VocabUnspecified, "ac-2"); err == nil {
		t.Error("an unresolved vocabulary must be rejected, not defaulted to NIST")
	}
	if _, err := FamilyID(VocabUnspecified, "ac"); err == nil {
		t.Error("an unresolved vocabulary must be rejected for family ids too")
	}
	if got := VocabUnspecified.String(); got != "unspecified" {
		t.Errorf("VocabUnspecified.String() = %q", got)
	}
	if got := VocabNIST80053.String(); got != "nist-sp800-53" {
		t.Errorf("VocabNIST80053.String() = %q", got)
	}
	if got := VocabOpaque.String(); got != "opaque" {
		t.Errorf("VocabOpaque.String() = %q", got)
	}
}

func TestFamilyID(t *testing.T) {
	for in, want := range map[string]string{"AC": "ac", "ac": "ac", "Si": "si"} {
		got, err := FamilyID(VocabNIST80053, in)
		if err != nil {
			t.Errorf("FamilyID(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("FamilyID(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{"ac-2", "A", "ABCD", "", "a1"} {
		if got, err := FamilyID(VocabNIST80053, in); err == nil {
			t.Errorf("FamilyID(%q) = %q, want an error", in, got)
		}
	}
	// Three letters are a family under the contract.
	if got, err := FamilyID(VocabNIST80053, "SAR"); err != nil || got != "sar" {
		t.Errorf("FamilyID(nist, \"SAR\") = %q, %v", got, err)
	}
	// And the held divergence: under an OPAQUE vocabulary a family is carried
	// as issued. SPARC's contract lowercases unconditionally, which is the
	// defect #37 removed from control ids — an AWS Security Hub family is
	// "ACM", and "acm" names nothing. See
	// docs/dev/sparc-family-id-normalisation.md.
	if got, err := FamilyID(VocabOpaque, "ACM"); err != nil || got != "ACM" {
		t.Errorf("FamilyID(opaque, \"ACM\") = %q, %v; want it carried as issued", got, err)
	}
	if got, err := FamilyID(VocabOpaque, "Effective Permissions"); err != nil || got != "Effective Permissions" {
		t.Errorf("FamilyID(opaque) = %q, %v", got, err)
	}
	if _, err := FamilyID(VocabOpaque, ""); err == nil {
		t.Error("an empty family id must be rejected under any vocabulary")
	}
}

// A field carrying the unit separator is rejected before any vocabulary rule
// runs, so the rejection cannot be bypassed by choosing a vocabulary that
// normalises less.
func TestSeparatorRejectedBeforeVocabularyRules(t *testing.T) {
	bad := "ACM" + Separator + ".1"
	if _, err := ControlID(VocabOpaque, bad); err == nil {
		t.Error("separator in an opaque control id must be rejected")
	}
	if _, err := FamilyID(VocabOpaque, bad); err == nil {
		t.Error("separator in an opaque family id must be rejected")
	}
}
