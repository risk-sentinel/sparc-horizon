// Package view computes API v0's read responses from a set of OSCAL documents.
//
// It is what lets Horizon show documents it did not write: an SSP, assessment
// plan, assessment results and POA&M from SPARC or any other OSCAL producer,
// joined through the relationships the OSCAL standard defines rather than
// through conventions only Horizon's fixture generator follows (#120).
//
// The joins, in the evidence chain's order (docs/03-data-model.md):
//
//   - assessment results → SSP: import-ap to the assessment plan, then its
//     import-ssp;
//   - POA&M → SSP: import-ssp, then system-id;
//   - finding → control: target.target-id, an objective or statement id;
//   - finding → observation → component: related-observations, then the
//     observation's subjects;
//   - finding → risk → POA&M item: related-risks, then the item's related
//     findings, risks or observations.
//
// A link that does not resolve is reported as a Problem and the document is
// still used where it can be. Nothing is dropped silently: a missing row looks
// identical to a clean one.
//
// This is not the projection engine. There is no ledger and nothing persists;
// state is computed from the documents at the horizon a request asks for. P2
// builds the rest.
package view

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-3"

	hoscal "github.com/risk-sentinel/sparc-horizon/internal/oscal"
)

// MaxDocumentBytes caps one file. A larger one is reported and skipped rather
// than read, because the mock reads documents a tester supplies.
const MaxDocumentBytes = 64 << 20

// Document is one OSCAL file, read and decoded.
type Document struct {
	// Path is relative to the root of the set, with forward slashes.
	Path    string
	Model   string
	Version string
	Bytes   []byte
	Doc     oscal.OscalCompleteSchema
}

// UUID is the document's own UUID, whatever its model.
func (d *Document) UUID() string {
	switch {
	case d.Doc.SystemSecurityPlan != nil:
		return d.Doc.SystemSecurityPlan.UUID
	case d.Doc.AssessmentPlan != nil:
		return d.Doc.AssessmentPlan.UUID
	case d.Doc.AssessmentResults != nil:
		return d.Doc.AssessmentResults.UUID
	case d.Doc.PlanOfActionAndMilestones != nil:
		return d.Doc.PlanOfActionAndMilestones.UUID
	case d.Doc.ComponentDefinition != nil:
		return d.Doc.ComponentDefinition.UUID
	}
	return ""
}

// stem is the file name without directory or extension.
func (d *Document) stem() string {
	base := path.Base(d.Path)
	return strings.TrimSuffix(base, path.Ext(base))
}

// Problem is something in the set that could not be read or joined.
type Problem struct {
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

// Set is every OSCAL document under one root.
type Set struct {
	Docs []*Document
	// NotOSCAL counts JSON files that are not an OSCAL model root (API
	// goldens, evidence files). They are counted, not listed: a fixtures
	// directory has a hundred of them and none is a problem.
	NotOSCAL int
	Problems []Problem
}

// LoadFS reads every *.json file under fsys.
func LoadFS(fsys fs.FS) (*Set, error) {
	files := map[string][]byte{}
	set := &Set{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".json") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxDocumentBytes {
			set.Problems = append(set.Problems, Problem{p, fmt.Sprintf("%d bytes, over the %d cap; skipped", info.Size(), MaxDocumentBytes)})
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		files[p] = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	loaded := LoadFiles(files)
	loaded.Problems = append(set.Problems, loaded.Problems...)
	return loaded, nil
}

// LoadFiles reads a set from file contents keyed by path.
func LoadFiles(files map[string][]byte) *Set {
	set := &Set{}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if !strings.HasSuffix(p, ".json") {
			continue
		}
		set.add(p, files[p])
	}
	return set
}

func (s *Set) add(p string, b []byte) {
	model, err := hoscal.Model(b)
	if err != nil {
		s.NotOSCAL++
		return
	}
	version, err := hoscal.Version(b)
	if err != nil {
		s.Problems = append(s.Problems, Problem{p, err.Error()})
		return
	}
	if !slices.Contains(hoscal.SupportedVersions, version) {
		s.Problems = append(s.Problems, Problem{p, fmt.Sprintf("OSCAL %s is not one of %v; skipped", version, hoscal.SupportedVersions)})
		return
	}
	// Decoded into the 1.2.3 types whatever the version declared. The fields
	// this package reads are unchanged across 1.1.2–1.2.3;
	// docs/dev/go-oscal-local-definitions.md records what the types drop.
	doc := &Document{Path: p, Model: model, Version: version, Bytes: b}
	if err := json.Unmarshal(b, &doc.Doc); err != nil {
		s.Problems = append(s.Problems, Problem{p, "does not decode: " + err.Error()})
		return
	}
	s.Docs = append(s.Docs, doc)
}

func (s *Set) ofModel(model string) []*Document {
	var out []*Document
	for _, d := range s.Docs {
		if d.Model == model {
			out = append(out, d)
		}
	}
	return out
}

func (s *Set) note(p, format string, args ...any) {
	s.Problems = append(s.Problems, Problem{p, fmt.Sprintf(format, args...)})
}
