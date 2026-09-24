package tree

import (
	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-2"
)

// bindRoles attaches this document's responsible-parties to the right tier.
//
// The tier is decided by the ROLE, not by member-of-organizations: every role
// holder in the fixtures is a member of the organization, so membership alone
// cannot tell an AO from an ISSO. docs/03-data-model.md is the authority —
// "AO in responsible-parties" for the organization tier, "SO and ISO in
// responsible-parties" for the boundary.
func (b *builder) bindRoles(ssp *oscal.SystemSecurityPlan, parties map[string]oscal.Party, org, boundary *Node, src string) {
	for _, rp := range responsibleParties(ssp) {
		role, ok := knownRole(rp.RoleId)
		if !ok {
			// Carrying it would put a value in the response that the frozen
			// schema rejects, so it is reported instead.
			b.note(FindingUnknownRole, src, "role-id %q is not one the contract enumerates", rp.RoleId)
			continue
		}

		target := boundary
		if role == AuthorizingOfficial {
			target = org
		}

		for _, uuid := range rp.PartyUuids {
			if _, known := parties[uuid]; !known {
				b.note(FindingUnknownParty, src,
					"responsible-party %s for role %q is not declared in parties", uuid, rp.RoleId)
				continue
			}
			addRole(target, role)
			b.bind(target, role, uuid)
		}
	}
}

// systems builds the system tier.
//
// A system is a COMPONENT THAT HAS AN INVENTORY RECORD. That uses both halves
// of what docs/03-data-model.md names — "SSP `components` and
// `inventory-items`" — and it is the rule the frozen API contract already
// encodes: the persona goldens in fixtures/api/ identify a system node by its
// COMPONENT uuid and title.
//
// #74 built this tier from inventory items instead, keyed on the item uuid and
// named by its `asset-id` prop. The count was right — 20 either way — so every
// structural test passed, and the identity was wrong. It was caught by
// internal/authz failing to reproduce goldens frozen in #61, which is the value
// of having a consumer that was specified before the implementation.
//
// Excluding components with no inventory record is not a special case for the
// inherited platform; it is the rule. The AWS component is not inventoried
// because it is not Horizon's system to inventory, which is exactly what makes
// it not a node.
func (b *builder) systems(ssp *oscal.SystemSecurityPlan, src string) []*Node {
	impl := ssp.SystemImplementation
	components := componentIndex(impl.Components)

	// Component uuid -> the roles reached through its inventory record. An
	// inventory item carries no responsible-parties of its own.
	inventoried := map[string]bool{}
	for _, item := range inventoryItems(impl) {
		b.recordInventory(item, components, inventoried, src)
	}

	var out []*Node
	for _, comp := range impl.Components {
		if !inventoried[comp.UUID] {
			continue
		}
		node := &Node{
			ID:       comp.UUID,
			Name:     comp.Title,
			NodeType: System,
			Roles:    []Role{},
		}
		b.componentRoles(comp, node, src)
		out = append(out, node)
	}
	return out
}

// recordInventory marks the component an inventory item stands for, and
// reports an item that stands for nothing.
func (b *builder) recordInventory(item oscal.InventoryItem, components map[string]oscal.SystemComponent, inventoried map[string]bool, src string) {
	impls := implementedComponents(item)
	if len(impls) == 0 {
		b.note(FindingOrphanInventory, src,
			"inventory item %s implements no component, so it names no system", item.UUID)
		return
	}
	for _, ic := range impls {
		if _, ok := components[ic.ComponentUuid]; !ok {
			b.note(FindingOrphanInventory, src,
				"inventory item %s implements component %s, which the document does not declare",
				item.UUID, ic.ComponentUuid)
			continue
		}
		inventoried[ic.ComponentUuid] = true
	}
}

// componentRoles attaches the roles the component declares. docs/03-data-model.md
// makes component `responsible-roles` the system tier's source.
func (b *builder) componentRoles(comp oscal.SystemComponent, node *Node, src string) {
	for _, rr := range responsibleRoles(comp) {
		role, known := knownRole(rr.RoleId)
		if !known {
			b.note(FindingUnknownRole, src,
				"component %s carries role-id %q, which the contract does not enumerate",
				comp.UUID, rr.RoleId)
			continue
		}
		addRole(node, role)
		// The component holds the role, so the component is the party.
		b.bind(node, role, comp.UUID)
	}
}
