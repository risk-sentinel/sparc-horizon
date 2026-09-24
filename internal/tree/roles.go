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
		}
	}
}

// systems builds the system tier from inventory-items.
//
// Inventory items, NOT components: the federation's 20 systems are its 20
// inventory items, while its 27 components include the inherited platform and
// other implementation components. Building from components would produce a
// tier that does not describe the systems (#74).
func (b *builder) systems(ssp *oscal.SystemSecurityPlan, src string) []*Node {
	impl := ssp.SystemImplementation
	components := componentIndex(impl.Components)

	var out []*Node
	for _, item := range inventoryItems(impl) {
		node := &Node{
			ID:       item.UUID,
			Name:     assetID(item),
			NodeType: System,
			Roles:    []Role{},
		}
		b.systemRoles(item, components, node, src)
		out = append(out, node)
	}
	return out
}

// systemRoles reaches the system tier's roles the only way OSCAL offers: an
// inventory item carries no responsible-parties of its own, so the roles come
// from the component it implements (docs/03-data-model.md, "Component
// responsible-roles").
func (b *builder) systemRoles(item oscal.InventoryItem, components map[string]oscal.SystemComponent, node *Node, src string) {
	impls := implementedComponents(item)
	if len(impls) == 0 {
		b.note(FindingOrphanInventory, src,
			"inventory item %s implements no component, so it can carry no role", item.UUID)
		return
	}

	for _, ic := range impls {
		comp, ok := components[ic.ComponentUuid]
		if !ok {
			b.note(FindingOrphanInventory, src,
				"inventory item %s implements component %s, which the document does not declare",
				item.UUID, ic.ComponentUuid)
			continue
		}
		for _, rr := range responsibleRoles(comp) {
			if role, known := knownRole(rr.RoleId); known {
				addRole(node, role)
			} else {
				b.note(FindingUnknownRole, src,
					"component %s carries role-id %q, which the contract does not enumerate",
					comp.UUID, rr.RoleId)
			}
		}
	}
}
