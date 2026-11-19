package view

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-3"

	"github.com/risk-sentinel/sparc-horizon/internal/authz"
	"github.com/risk-sentinel/sparc-horizon/internal/tree"
)

// DateLayout is the contract's date format.
const DateLayout = "2006-01-02"

// Options adjust how a set is read.
type Options struct {
	// RoleAliases maps a document's role-id onto a contract role. See
	// tree.Options.
	RoleAliases map[string]tree.Role
}

// View answers the read endpoints over one document set.
type View struct {
	set        *Set
	links      *linker
	tree       tree.Result
	authz      *authz.Authorizer
	nodes      map[string]*tree.Node
	cdefReqs   map[string]string
	assessed   map[string][]assessment // boundary node id -> assessments
	byDocument map[string]*Document    // export href -> document
	docBound   map[string]*boundary    // export href -> its boundary
}

// New joins a set. It fails only when no SSP can be built into a tree; every
// lesser problem is in Problems.
func New(set *Set, opts Options) (*View, error) {
	v := &View{
		set:        set,
		nodes:      map[string]*tree.Node{},
		cdefReqs:   map[string]string{},
		assessed:   map[string][]assessment{},
		byDocument: map[string]*Document{},
		docBound:   map[string]*boundary{},
	}
	v.links = link(set)
	if len(v.links.boundaries) == 0 {
		return nil, fmt.Errorf("view: no system-security-plan in the set")
	}

	if err := v.buildTree(opts); err != nil {
		return nil, err
	}
	v.indexComponentDefinitions()
	for _, b := range v.links.boundaries {
		v.assessed[b.id()] = v.assess(b)
		for _, d := range v.exportDocs(b) {
			v.byDocument[hrefOf(d)] = d
			v.docBound[hrefOf(d)] = b
		}
	}
	return v, nil
}

// buildTree builds the tree and authorizer from the SSPs, reporting what did
// not join.
func (v *View) buildTree(opts Options) error {
	plans := make([]*oscal.SystemSecurityPlan, 0, len(v.links.boundaries))
	for _, b := range v.links.boundaries {
		plans = append(plans, b.plan)
	}
	res, err := tree.BuildWith(plans, tree.Options{RoleAliases: opts.RoleAliases})
	if err != nil {
		return fmt.Errorf("view: %w", err)
	}
	for _, f := range res.Findings {
		v.set.note(f.Source, "tree: %s: %s", f.Kind, f.Detail)
	}
	v.tree = res
	if v.authz, err = authz.New(res); err != nil {
		return fmt.Errorf("view: %w", err)
	}
	index(res.Root, v.nodes)
	return nil
}

// indexComponentDefinitions keys each component definition's requirements by
// control, for controls that reach a boundary only through one.
func (v *View) indexComponentDefinitions() {
	for _, d := range v.set.ofModel(modelCDef) {
		for _, c := range deref(d.Doc.ComponentDefinition.Components) {
			for _, ci := range deref(c.ControlImplementations) {
				v.indexRequirements(ci.ImplementedRequirements)
			}
		}
	}
}

func (v *View) indexRequirements(irs []oscal.ImplementedRequirementControlImplementation) {
	for _, ir := range irs {
		if _, ok := v.cdefReqs[normal(ir.ControlId)]; !ok {
			v.cdefReqs[normal(ir.ControlId)] = ir.UUID
		}
	}
}

func index(n *tree.Node, into map[string]*tree.Node) {
	if n == nil {
		return
	}
	into[n.ID] = n
	for _, c := range n.Children {
		index(c, into)
	}
}

// Problems is everything in the set that did not read or join.
func (v *View) Problems() []Problem { return v.set.Problems }

// Tree is the whole tree, its bindings and findings.
func (v *View) Tree() tree.Result { return v.tree }

// Authorizer decides visibility for the set.
func (v *View) Authorizer() *authz.Authorizer { return v.authz }

// Summary describes what loaded, for a startup log.
func (v *View) Summary() string {
	counts := map[string]int{}
	for _, d := range v.set.Docs {
		counts[d.Model]++
	}
	placed := 0
	for _, b := range v.links.boundaries {
		placed += len(v.assessed[b.id()])
	}
	return fmt.Sprintf("%d SSP, %d assessment plan, %d assessment results, %d POA&M, %d component definition; "+
		"%d boundaries, %d assessed control-component pairs, %d other JSON files ignored, %d problems",
		counts[modelSSP], counts[modelAP], counts[modelAR],
		counts[modelPOAM], counts[modelCDef],
		len(v.links.boundaries), placed, v.set.NotOSCAL, len(v.set.Problems))
}

// visible reports whether party may see node.
func (v *View) visible(party, node string) bool { return v.authz.Visible(party, node) }

// boundariesUnder is the boundaries beneath a node, in tree order, or nil for
// a node that is neither an organization nor a boundary.
func (v *View) boundariesUnder(n *tree.Node) []*boundary {
	switch n.NodeType {
	case tree.Boundary:
		if b := v.links.byNode[n.ID]; b != nil {
			return []*boundary{b}
		}
		return nil
	case tree.Organization:
		var out []*boundary
		for _, c := range n.Children {
			if b := v.links.byNode[c.ID]; b != nil {
				out = append(out, b)
			}
		}
		return out
	}
	return nil
}

// Chain is the evidence chain for an implemented requirement.
func (v *View) Chain(party, requirement string) (Chain, bool) {
	for _, b := range v.links.boundaries {
		for _, a := range v.assessed[b.id()] {
			if a.requirement != requirement {
				continue
			}
			// A requirement is visible when its component is; one about no
			// node in the tree (an uninventoried component, or none) is
			// visible with its boundary.
			subject := a.component
			if _, isNode := v.nodes[subject]; !isNode {
				subject = b.id()
			}
			// A requirement from a shared component definition recurs in
			// every boundary that inherits it; the caller gets the first
			// one they can see.
			if v.visible(party, subject) {
				return chainOf(a), true
			}
		}
	}
	return Chain{}, false
}

func chainOf(a assessment) Chain {
	c := Chain{
		Requirement: a.requirement,
		ControlID:   a.controlID,
		Component:   a.component,
		Resource:    a.resource,
		Observation: a.observation,
		Finding:     a.finding,
		Risk:        a.risk,
		BlocksATO:   a.blocksATO,
		PoamItem:    a.poamItem,
	}
	if a.expires != nil {
		c.Expires = a.expires.Format(DateLayout)
	}
	return c
}

// exportDocs are a boundary's SSP, assessment results and POA&Ms, in that
// order.
func (v *View) exportDocs(b *boundary) []*Document {
	out := []*Document{b.ssp}
	out = append(out, b.ars...)
	return append(out, b.poams...)
}

// hrefOf is where the mock serves a document: under /oscal/, with a leading
// "oscal/" directory folded in so the fixtures keep their paths.
func hrefOf(d *Document) string {
	return "/oscal/" + strings.TrimPrefix(d.Path, "oscal/")
}

// Export lists a visible boundary's documents, named by slug or SSP UUID.
func (v *View) Export(party, ref string) (Export, bool) {
	b := v.boundaryByRef(ref)
	if b == nil || !v.visible(party, b.id()) {
		return Export{}, false
	}
	out := Export{Boundary: b.slug, Documents: []ExportDocument{}}
	for _, d := range v.exportDocs(b) {
		sum := sha256.Sum256(d.Bytes)
		out.Documents = append(out.Documents, ExportDocument{Model: d.Model, Href: hrefOf(d), SHA256: hex.EncodeToString(sum[:])})
	}
	return out, true
}

func (v *View) boundaryByRef(ref string) *boundary {
	for _, b := range v.links.boundaries {
		if b.slug == ref || strings.EqualFold(b.id(), ref) || strings.EqualFold(b.plan.UUID, ref) {
			return b
		}
	}
	return nil
}

// Document returns an exported document's bytes, unchanged, when its boundary
// is visible. href is the path an Export listed.
func (v *View) Document(party, href string) ([]byte, bool) {
	d, ok := v.byDocument[href]
	if !ok || !v.visible(party, v.docBound[href].id()) {
		return nil, false
	}
	return d.Bytes, true
}

// ParseHorizon reads the contract's horizon parameter; empty means def.
func ParseHorizon(s string, def time.Time) (time.Time, error) {
	if s == "" {
		return def, nil
	}
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("horizon must be YYYY-MM-DD: %w", err)
	}
	return t, nil
}

// Chains is every assessment's chain, unscoped, for the fixture generator's
// goldens. The mock answers through Chain, which applies visibility.
func (v *View) Chains() []Chain {
	var out []Chain
	for _, b := range v.links.boundaries {
		for _, a := range v.assessed[b.id()] {
			out = append(out, chainOf(a))
		}
	}
	return out
}
