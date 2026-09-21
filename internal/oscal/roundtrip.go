package oscal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// The round-trip fidelity probe from #26: unmarshal a document into the typed
// structs, marshal it back, and compare every path against the source.
//
// Compilation was never the question. Horizon's audit test is that every cell
// at every tier recomputes identically from exported OSCAL alone, so the
// question is whether a document survives the trip.

// DiffKind separates the one difference #26 expects from every other.
type DiffKind string

const (
	// DiffTimestamp is a timestamp that named the same instant before and
	// after, in different bytes — "+00:00" becoming "Z" as it passes through
	// time.Time.
	//
	// Harmless for computation and fatal for signatures: detached signatures
	// are over JCS-canonical (RFC 8785) JSON, and JCS canonicalises member
	// order and number formatting but not string values. A timestamp is a
	// string, so a parse-and-re-serialise cycle changes the digest of a
	// document nobody edited. Hence the standing rule from #26: verify and
	// hash over the received bytes, never over a re-serialisation.
	DiffTimestamp DiffKind = "timestamp-normalisation"

	// DiffOther is loss, gain or change. It is what fails a test.
	DiffOther DiffKind = "difference"
)

// Difference is one path where the re-serialised document departs from the
// source.
type Difference struct {
	Path   string   `json:"path"`
	Kind   DiffKind `json:"kind"`
	Source string   `json:"source"`
	After  string   `json:"after"`
}

func (d Difference) String() string {
	return fmt.Sprintf("%s\n  source: %s\n  after:  %s", d.Path, d.Source, d.After)
}

// Only returns the differences of one kind.
func Only(ds []Difference, kind DiffKind) []Difference {
	var out []Difference
	for _, d := range ds {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}

// RoundTrip unmarshals a document into ts's typed structs, marshals it back,
// and returns every difference from the source.
func RoundTrip(b []byte, ts TypeSet) ([]Difference, error) {
	doc := ts.newDocument()
	if err := json.Unmarshal(b, doc); err != nil {
		return nil, fmt.Errorf("oscal: unmarshal at %s: %w", ts.Version, err)
	}
	after, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("oscal: marshal at %s: %w", ts.Version, err)
	}

	source, err := decodeGeneric(b)
	if err != nil {
		return nil, err
	}
	reserialised, err := decodeGeneric(after)
	if err != nil {
		return nil, err
	}

	var diffs []Difference
	compare("", source, reserialised, &diffs)
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Path < diffs[j].Path })
	return diffs, nil
}

// decodeGeneric decodes into plain Go values with numbers left as json.Number,
// so that 1 and 1.0 are not reported as a difference and a large integer does
// not lose precision through float64 on the way to being compared.
func decodeGeneric(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("oscal: decoding for comparison: %w", err)
	}
	return v, nil
}

func compare(path string, source, after any, out *[]Difference) {
	switch src := source.(type) {
	case map[string]any:
		aft, ok := after.(map[string]any)
		if !ok {
			record(path, source, after, out)
			return
		}
		for _, k := range union(src, aft) {
			s, inSource := src[k]
			a, inAfter := aft[k]
			switch {
			case inSource && !inAfter:
				record(path+"/"+k, s, nil, out)
			case !inSource && inAfter:
				record(path+"/"+k, nil, a, out)
			default:
				compare(path+"/"+k, s, a, out)
			}
		}
	case []any:
		aft, ok := after.([]any)
		if !ok || len(src) != len(aft) {
			record(path, source, after, out)
			return
		}
		for i := range src {
			compare(fmt.Sprintf("%s/%d", path, i), src[i], aft[i], out)
		}
	default:
		if !scalarEqual(source, after) {
			record(path, source, after, out)
		}
	}
}

func union(a, b map[string]any) []string {
	seen := map[string]bool{}
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func scalarEqual(a, b any) bool {
	an, aIsNum := a.(json.Number)
	bn, bIsNum := b.(json.Number)
	if aIsNum && bIsNum {
		return an.String() == bn.String()
	}
	return a == b
}

func record(path string, source, after any, out *[]Difference) {
	d := Difference{
		Path:   path,
		Kind:   DiffOther,
		Source: render(source),
		After:  render(after),
	}
	if sameInstant(source, after) {
		d.Kind = DiffTimestamp
	}
	*out = append(*out, d)
}

// sameInstant reports whether two values are timestamps naming the same moment
// in different text. This is a classification, not a tolerance: the difference
// is still reported, with the kind that says why it is expected.
func sameInstant(a, b any) bool {
	as, aok := a.(string)
	bs, bok := b.(string)
	if !aok || !bok || as == bs {
		return false
	}
	at, err := time.Parse(time.RFC3339Nano, as)
	if err != nil {
		return false
	}
	bt, err := time.Parse(time.RFC3339Nano, bs)
	if err != nil {
		return false
	}
	return at.Equal(bt)
}

func render(v any) string {
	if v == nil {
		return "<absent>"
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	s := string(b)
	// A whole subtree renders as one line; truncate it so a report stays
	// readable, because the path is what locates the problem.
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return strings.TrimSpace(s)
}

// Measurement is one document probed against one version's types: what the
// round trip lost, what it merely reformatted, and what the types never
// modelled at all.
type Measurement struct {
	TypeVersion          string   `json:"type-version"`
	Differences          int      `json:"differences"`
	TimestampDifferences int      `json:"timestamp-differences"`
	StrictDecodeError    string   `json:"strict-decode-error,omitempty"`
	Paths                []string `json:"paths,omitempty"`
}

// Probe measures a document against every version Horizon decodes, not only
// the one it declares.
//
// Cross-version decoding is measured rather than assumed because published
// content does not agree on a version: NIST's own examples declare 1.1.2,
// 1.1.3 and 1.2.2 across the models Horizon reads, and a peer may send either.
func Probe(b []byte) []Measurement {
	out := make([]Measurement, 0, len(SupportedVersions))
	for _, v := range SupportedVersions {
		ts, err := TypesFor(v)
		if err != nil {
			continue
		}
		m := Measurement{TypeVersion: v}
		diffs, err := RoundTrip(b, ts)
		if err != nil {
			m.StrictDecodeError = err.Error()
			out = append(out, m)
			continue
		}
		for _, d := range diffs {
			if d.Kind == DiffTimestamp {
				m.TimestampDifferences++
				continue
			}
			m.Differences++
			m.Paths = append(m.Paths, d.Path)
		}
		if err := StrictDecode(b, ts); err != nil {
			m.StrictDecodeError = err.Error()
		}
		out = append(out, m)
	}
	return out
}
