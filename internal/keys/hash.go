package keys

import (
	"fmt"
	"regexp"
	"strings"
)

var sha256Hex = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// canonicalSHA256 normalises a content hash to lowercase hex. Case is the only
// thing normalised; a value that is not a SHA-256 digest is rejected, because
// an evidence resource keyed on something other than the hash of its content
// is not the object this grammar describes.
func canonicalSHA256(v string) (string, error) {
	if !sha256Hex.MatchString(v) {
		return "", fmt.Errorf("keys: %q is not a SHA-256 hex digest", v)
	}
	return strings.ToLower(v), nil
}
