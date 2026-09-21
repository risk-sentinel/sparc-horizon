// Package canonical normalises the field values that go into a deterministic
// identifier, so two independent implementations produce identical bytes for
// the same logical key.
//
// The rules here are normative in docs/03-data-model.md. They are separated
// from the key grammar itself deliberately: this package says what a field
// value looks like, while internal/keys says which fields make up a key.
//
// Control identifiers are canonicalised in controlid.go, and only within a
// stated vocabulary. #37 settled that scoping — `source-uuid` is what makes
// the vocabulary decidable — so the rule this package once deferred is now
// implemented rather than absent.
package canonical

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Separator joins fields in a natural key. It is the ASCII unit separator
// rather than a printable delimiter because a printable one is ambiguous:
// joining with "|", the tuples ("a|b", "c") and ("a", "b|c") produce the same
// input and therefore the same UUID. A collision by accident is harder to
// notice than one by attack.
const Separator = "\x1f"

// ErrSeparatorInField reports a field value containing Separator.
//
// The value is rejected rather than escaped. Escaping re-introduces the
// ambiguity it is meant to remove, because an escape sequence is itself a
// value a field could legitimately contain. Rejection makes the collision
// impossible rather than unlikely.
var ErrSeparatorInField = errors.New("canonical: field contains the unit separator")

var (
	// YYYY | YYYY-Qn | YYYY-MM | YYYY-MM-DD, zero-padded throughout.
	periodRe = regexp.MustCompile(`^[0-9]{4}(-(Q[1-4]|(0[1-9]|1[0-2])(-(0[1-9]|[12][0-9]|3[01]))?))?$`)
	// RFC 9562 §4 textual form, case-insensitive on input.
	uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// ValidateField reports whether a value may appear in a natural key.
//
// NIST SP 800-53 SI-10 (information input validation): every field that
// reaches a derivation passes through here or through one of the typed
// canonicalisers below, and a value that does not conform is rejected rather
// than repaired.
func ValidateField(v string) error {
	if strings.Contains(v, Separator) {
		return fmt.Errorf("%w: %q", ErrSeparatorInField, v)
	}
	return nil
}

// Join validates every field and joins them with Separator. It returns an
// error rather than a best-effort string: a key derived from an invalid field
// is a silent data-integrity fault, not a recoverable one.
func Join(fields ...string) (string, error) {
	for i, f := range fields {
		if err := ValidateField(f); err != nil {
			return "", fmt.Errorf("field %d: %w", i, err)
		}
	}
	return strings.Join(fields, Separator), nil
}

// Period canonicalises a period label. Quarters are "2026-Q3", never "2026Q3"
// or "Q3-2026", and every numeric part is zero-padded, so "2026-1" is not a
// spelling of "2026-01" but a rejection.
func Period(v string) (string, error) {
	if !periodRe.MatchString(v) {
		return "", fmt.Errorf("canonical: %q is not a period label", v)
	}
	return v, nil
}

// UUID canonicalises a UUID to lowercase hex with hyphens. Case is the only
// thing normalised; a malformed value is rejected rather than repaired.
func UUID(v string) (string, error) {
	if !uuidRe.MatchString(v) {
		return "", fmt.Errorf("canonical: %q is not a UUID", v)
	}
	return strings.ToLower(v), nil
}

// String canonicalises any other string field: Unicode NFC, no trimming and
// no case folding. A field that needs case folding to match is the wrong
// field — folding "Ac-2" into a key means the key depends on how someone
// typed it.
func String(v string) string {
	return norm.NFC.String(v)
}
