package canonical

import (
	"fmt"
	"regexp"
	"strings"
)

// Vocabulary names the catalog authority a control identifier was issued by.
//
// Canonicalisation is valid only within a vocabulary. Lowercasing the AWS
// Security Hub identifier "ACM.1" yields "acm.1", which looks like a NIST
// control and is not one — a value that still validates, still derives a UUID,
// and names nothing. That is worse than a rejection, because nothing
// downstream can tell.
//
// Which vocabulary applies is decided by the `source-uuid` on the key: the
// UUID of the catalog or profile that defines the control. A caller that has
// not resolved one has nothing to decide with, which is why the zero value is
// invalid rather than a default.
type Vocabulary int

const (
	// VocabUnspecified is the zero value and is always rejected. A default
	// here would silently canonicalise another authority's identifier into
	// NIST's form the first time someone forgot to resolve `source-uuid`.
	VocabUnspecified Vocabulary = iota
	// VocabNIST80053 is NIST SP 800-53, whose canonical form is OSCAL's
	// lowercase dotted "ac-2.1".
	VocabNIST80053
	// VocabOpaque is every other authority: AWS Security Hub, CIS, a vendor
	// benchmark. The identifier is NFC-normalised and otherwise carried
	// exactly as issued.
	VocabOpaque
)

func (v Vocabulary) String() string {
	switch v {
	case VocabNIST80053:
		return "nist-sp800-53"
	case VocabOpaque:
		return "opaque"
	default:
		return "unspecified"
	}
}

// ParseVocabulary reads a vocabulary back from the name String reports. It
// exists because the published test vectors carry the vocabulary as a string,
// and a consumer in any language has to reach the same rule the deriver used.
func ParseVocabulary(s string) (Vocabulary, error) {
	switch s {
	case VocabNIST80053.String():
		return VocabNIST80053, nil
	case VocabOpaque.String():
		return VocabOpaque, nil
	case "", VocabUnspecified.String():
		return VocabUnspecified, nil
	default:
		return VocabUnspecified, fmt.Errorf("canonical: %q is not a vocabulary", s)
	}
}

// nistControl matches the spellings SPARC's API and the source documents have
// both been observed to issue: "AC-2", "ac-2", "AC-02", "AC-2.1", "AC-2(1)"
// and "AC-2 (1)". Leading zeros are absorbed here rather than stripped later,
// so "AC-02" and "AC-2" cannot take different paths through the parser.
//
// The prefix is two or three letters and enhancements nest, because SPARC's
// shared contract canonicalises to `^[a-z]{2,3}-\d+(\.\d+)*$` and an
// implementation stricter than the shared grammar rejects keys a peer
// legitimately derives.
var nistControl = regexp.MustCompile(`^([A-Za-z]{2,3})-0*([1-9][0-9]{0,2})((?:\.0*[1-9][0-9]{0,2}|[ ]?\(0*[1-9][0-9]{0,2}\))*)$`)

// nistEnhancement pulls each level out of the tail nistControl captured, in
// whichever of the two spellings it arrived in.
var nistEnhancement = regexp.MustCompile(`\.0*([1-9][0-9]{0,2})|\(0*([1-9][0-9]{0,2})\)`)

var nistFamily = regexp.MustCompile(`^[A-Za-z]{2,3}$`)

// ControlID canonicalises a control identifier within its issuing vocabulary.
//
// Under VocabNIST80053 the result is OSCAL's lowercase dotted form: "ac-2",
// "ac-2.1". Enhancements use ".", never parentheses, because `control-id` is
// TokenDatatype in six OSCAL schemas and the dotted form is what those schemas
// accept.
//
// Under VocabOpaque the result is the input in Unicode NFC. No case folding,
// no separator rewriting, no zero stripping: "ACM.1" stays "ACM.1" and the CIS
// identifier "1.1.1" stays "1.1.1".
//
// A statement fragment such as "ac-2_smt.a" is rejected under both. It names a
// part of a control, not a control, and a key that cannot tell the two apart
// counts one object twice.
func ControlID(v Vocabulary, id string) (string, error) {
	s := String(id)
	if err := ValidateField(s); err != nil {
		return "", err
	}
	if s == "" {
		return "", fmt.Errorf("canonical: empty control id")
	}

	switch v {
	case VocabNIST80053:
		m := nistControl.FindStringSubmatch(s)
		if m == nil {
			return "", fmt.Errorf("canonical: %q is not an SP 800-53 control id", id)
		}
		out := strings.ToLower(m[1]) + "-" + m[2]
		// m[3] is the whole enhancement tail, in either spelling and to any
		// depth. Each level is emitted dotted, which is the form OSCAL's
		// TokenDatatype accepts.
		for _, e := range nistEnhancement.FindAllStringSubmatch(m[3], -1) {
			level := e[1]
			if level == "" {
				level = e[2]
			}
			out += "." + level
		}
		return out, nil
	case VocabOpaque:
		return s, nil
	default:
		return "", fmt.Errorf("canonical: no vocabulary given for control id %q", id)
	}
}

// FamilyID canonicalises a control family identifier. Projection cells key on
// a control id or a family id in the same field, so the family form needs the
// same treatment as the control form or half the cells in a rollup are keyed
// inconsistently with the other half.
func FamilyID(v Vocabulary, id string) (string, error) {
	s := String(id)
	if err := ValidateField(s); err != nil {
		return "", err
	}
	if s == "" {
		return "", fmt.Errorf("canonical: empty family id")
	}

	switch v {
	case VocabNIST80053:
		if !nistFamily.MatchString(s) {
			return "", fmt.Errorf("canonical: %q is not an SP 800-53 family id", id)
		}
		return strings.ToLower(s), nil
	case VocabOpaque:
		return s, nil
	default:
		return "", fmt.Errorf("canonical: no vocabulary given for family id %q", id)
	}
}
