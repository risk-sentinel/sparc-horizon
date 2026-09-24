package tree

import (
	"fmt"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"
)

// builder accumulates nodes across the SSP set. Organizations and the
// federation are shared between SSPs, so they are keyed by party UUID and
// visited repeatedly; boundaries and systems belong to exactly one document.
type builder struct {
	federation *Node
	orgs       map[string]*Node
	orgOrder   []string
	findings   []Finding
	seenSystem map[string]string // system-id -> the SSP that claimed it first
}

// Build assembles the tree from a set of boundary SSPs.
//
// The SSPs may arrive in any order and the result is stable: children are
// sorted by name then id, so a golden file compares byte for byte regardless
// of how the documents were loaded.
func Build(ssps []*oscal.SystemSecurityPlan) (Result, error) {
	if len(ssps) == 0 {
		return Result{}, fmt.Errorf("tree: no SSPs to build from")
	}

	b := &builder{
		orgs:       map[string]*Node{},
		seenSystem: map[string]string{},
	}

	for _, ssp := range ssps {
		if ssp == nil {
			return Result{}, fmt.Errorf("tree: nil SSP in the set")
		}
		b.addBoundary(ssp)
	}

	if b.federation == nil {
		return Result{}, fmt.Errorf("tree: no federation party in any SSP")
	}

	for _, uuid := range b.orgOrder {
		b.federation.Children = append(b.federation.Children, b.orgs[uuid])
	}
	sortTree(b.federation)

	return Result{Root: b.federation, Findings: b.findings}, nil
}

func (b *builder) note(kind, source, format string, args ...any) {
	b.findings = append(b.findings, Finding{
		Kind:   kind,
		Source: source,
		Detail: fmt.Sprintf(format, args...),
	})
}

// addBoundary folds one SSP into the tree.
func (b *builder) addBoundary(ssp *oscal.SystemSecurityPlan) {
	src := boundarySystemID(ssp)
	parties := partyIndex(ssp)

	org := b.resolveOrganization(parties, src)
	if org == nil {
		return
	}

	node := &Node{
		ID:       ssp.UUID,
		Name:     ssp.SystemCharacteristics.SystemName,
		NodeType: Boundary,
		Roles:    []Role{},
	}

	if first, dup := b.seenSystem[src]; dup && src != "" {
		b.note(FindingDuplicateSystemID, src,
			"system-id %q is claimed by SSP %s and again by %s", src, first, ssp.UUID)
	} else if src != "" {
		b.seenSystem[src] = ssp.UUID
	}

	b.bindRoles(ssp, parties, org, node, src)
	node.Children = append(node.Children, b.systems(ssp, src)...)
	org.Children = append(org.Children, node)
}

// resolveOrganization finds this SSP's organization node, creating it and the
// federation node on first sight. A nil return means the document could not be
// placed in the tree at all, which is reported rather than skipped silently.
func (b *builder) resolveOrganization(parties map[string]oscal.Party, src string) *Node {
	fed, org, err := tiers(parties)
	if err != nil {
		kind := FindingNoOrganization
		if fed.UUID == "" {
			kind = FindingNoFederation
		}
		b.note(kind, src, "%v", err)
		return nil
	}

	switch {
	case b.federation == nil:
		b.federation = &Node{ID: fed.UUID, Name: fed.Name, NodeType: Federation, Roles: []Role{}}
	case b.federation.ID != fed.UUID:
		// Two SSPs disagree about which federation they belong to. Joining
		// them anyway would invent a tree neither document describes.
		b.note(FindingFederationSplit, src,
			"SSP names federation %s, the tree is rooted at %s", fed.UUID, b.federation.ID)
		return nil
	}

	node, ok := b.orgs[org.UUID]
	if !ok {
		node = &Node{ID: org.UUID, Name: org.Name, NodeType: Organization, Roles: []Role{}}
		b.orgs[org.UUID] = node
		b.orgOrder = append(b.orgOrder, org.UUID)
	}
	return node
}

// tiers picks the federation and organization parties out of one document.
//
// The federation is the organization party that is a member of nothing; an
// organization is a party that is a member of it. Both are read from the SSP
// rather than from an admin table, because a document edit is the grant.
func tiers(parties map[string]oscal.Party) (fed, org oscal.Party, err error) {
	for _, p := range parties {
		if p.Type != "organization" {
			continue
		}
		if len(memberOf(p)) == 0 {
			fed = p
			continue
		}
		org = p
	}

	if fed.UUID == "" {
		return fed, org, fmt.Errorf("no organization party without member-of-organizations")
	}
	if org.UUID == "" {
		return fed, org, fmt.Errorf("no organization party beneath federation %s", fed.UUID)
	}
	return fed, org, nil
}
