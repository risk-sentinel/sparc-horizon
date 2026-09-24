package tree

import (
	"sort"

	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"
)

// go-oscal models optional OSCAL assemblies as pointers to slices, so every
// read is a nil check. These helpers do it once each, so the builder reads as
// the rules it implements rather than as pointer plumbing.

func memberOf(p oscal.Party) []string {
	if p.MemberOfOrganizations == nil {
		return nil
	}
	return *p.MemberOfOrganizations
}

func responsibleParties(ssp *oscal.SystemSecurityPlan) []oscal.ResponsibleParty {
	if ssp.Metadata.ResponsibleParties == nil {
		return nil
	}
	return *ssp.Metadata.ResponsibleParties
}

func responsibleRoles(c oscal.SystemComponent) []oscal.ResponsibleRole {
	if c.ResponsibleRoles == nil {
		return nil
	}
	return *c.ResponsibleRoles
}

func inventoryItems(impl oscal.SystemImplementation) []oscal.InventoryItem {
	if impl.InventoryItems == nil {
		return nil
	}
	return *impl.InventoryItems
}

func implementedComponents(item oscal.InventoryItem) []oscal.ImplementedComponent {
	if item.ImplementedComponents == nil {
		return nil
	}
	return *item.ImplementedComponents
}

func partyIndex(ssp *oscal.SystemSecurityPlan) map[string]oscal.Party {
	out := map[string]oscal.Party{}
	if ssp.Metadata.Parties == nil {
		return out
	}
	for _, p := range *ssp.Metadata.Parties {
		out[p.UUID] = p
	}
	return out
}

func componentIndex(components []oscal.SystemComponent) map[string]oscal.SystemComponent {
	out := make(map[string]oscal.SystemComponent, len(components))
	for _, c := range components {
		out[c.UUID] = c
	}
	return out
}

// boundarySystemID is SPARC's slug for the boundary, carried in the SSP's
// first system-id. It names the document in findings, because a UUID alone
// tells a reader nothing about which system is wrong.
func boundarySystemID(ssp *oscal.SystemSecurityPlan) string {
	if len(ssp.SystemCharacteristics.SystemIds) == 0 {
		return ssp.UUID
	}
	return ssp.SystemCharacteristics.SystemIds[0].ID
}

// assetID names a system node. docs/03-data-model.md makes the inventory
// `asset-id` the join key to an HDF target, so it is the name that means
// something to a reader; the description is prose written for a human.
func assetID(item oscal.InventoryItem) string {
	if item.Props != nil {
		for _, p := range *item.Props {
			if p.Name == "asset-id" {
				return p.Value
			}
		}
	}
	return item.UUID
}

// addRole keeps Roles a set: a role declared twice for one node is one
// binding, and duplicates would make the golden file depend on document order.
func addRole(n *Node, r Role) {
	for _, have := range n.Roles {
		if have == r {
			return
		}
	}
	n.Roles = append(n.Roles, r)
}

// sortTree makes the output stable regardless of the order SSPs were loaded
// in, which is what lets a golden file be compared byte for byte.
func sortTree(n *Node) {
	sort.Slice(n.Roles, func(i, j int) bool { return n.Roles[i] < n.Roles[j] })
	sort.Slice(n.Children, func(i, j int) bool {
		if n.Children[i].Name != n.Children[j].Name {
			return n.Children[i].Name < n.Children[j].Name
		}
		return n.Children[i].ID < n.Children[j].ID
	})
	for _, c := range n.Children {
		sortTree(c)
	}
}
