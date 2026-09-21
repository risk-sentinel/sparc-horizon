package keys

import (
	"fmt"
	"regexp"

	"github.com/risk-sentinel/sparc-horizon/internal/canonical"
)

// Bucket is the horizon a projection cell was materialised for.
//
// docs/05-projection-engine.md materialises today, +7, +14, +30 and every
// decision date; arbitrary slider values are computed on demand from the
// nearest bucket and are never keyed. The fixed tokens and a decision date are
// therefore the whole vocabulary, and anything else is rejected rather than
// hashed — a cell keyed on a value the engine cannot materialise is a cell
// nothing will ever invalidate.
type Bucket string

const (
	BucketToday  Bucket = "today"
	BucketPlus7  Bucket = "+7"
	BucketPlus14 Bucket = "+14"
	BucketPlus30 Bucket = "+30"
)

// A decision date, which is a full calendar date and never a quarter or a
// month: a decision happens on a day.
var bucketDate = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

// BucketOnDate returns the bucket for a decision date, in YYYY-MM-DD.
func BucketOnDate(date string) (Bucket, error) {
	if !bucketDate.MatchString(date) {
		return "", fmt.Errorf("keys: %q is not a decision date (want YYYY-MM-DD)", date)
	}
	// Reuse the period rules so a bucket date and a period date cannot be
	// validated by two different notions of what a date is.
	if _, err := canonical.Period(date); err != nil {
		return "", err
	}
	return Bucket(date), nil
}

func (b Bucket) canonical() (string, error) {
	switch b {
	case BucketToday, BucketPlus7, BucketPlus14, BucketPlus30:
		return string(b), nil
	}
	if _, err := BucketOnDate(string(b)); err != nil {
		return "", fmt.Errorf("keys: %q is not a horizon bucket", b)
	}
	return string(b), nil
}
