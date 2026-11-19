package tree

import (
	oscal "github.com/defenseunicorns/go-oscal/src/types/oscal-1-2-3"

	"github.com/risk-sentinel/sparc-horizon/internal/keys"
)

// A boundary's node id (#124).
//
// SPARC holds one SSP per authorization boundary and exports it with
// `node-type: system` and `parent-uuid` set to the boundary's own uuid on the
// SSP's metadata (sparc PR #1197, owner decision 2026-09-29). That uuid is
// stable; the SSP document's uuid is not. OSCAL lets a producer change a
// document's uuid on any revision, so a node keyed by it can change id when
// nothing about the boundary changed, and every link to it breaks.
//
// So the boundary node takes the SPARC boundary uuid when the SSP carries
// SPARC's form, and the SSP's uuid otherwise: Horizon's fixtures, which use
// `node-type: boundary` with `parent-uuid` naming the organization, and any
// producer that writes neither prop.

// sparcBoundaryUUID returns the boundary uuid an SSP declares in SPARC's form,
// or "" when it does not declare one.
func sparcBoundaryUUID(ssp *oscal.SystemSecurityPlan) string {
	if ssp == nil || ssp.Metadata.Props == nil {
		return ""
	}
	var nodeType, parent string
	for _, p := range *ssp.Metadata.Props {
		if p.Ns != keys.NamespaceURI {
			continue
		}
		switch p.Name {
		case "node-type":
			nodeType = p.Value
		case "parent-uuid":
			parent = p.Value
		}
	}
	if nodeType != "system" {
		return ""
	}
	return parent
}

// BoundaryIDs maps each SSP's uuid to its boundary node id. A boundary uuid
// claimed by more than one SSP in the set is used by none of them: each falls
// back to its own document uuid, so two boundaries never share a node, and the
// claim is returned so the tree can report it. The view calls this too, over
// the same set, so the tree and the heat rows agree on every id.
func BoundaryIDs(ssps []*oscal.SystemSecurityPlan) (ids map[string]string, contested map[string][]string) {
	claims := map[string][]string{}
	for _, ssp := range ssps {
		if b := sparcBoundaryUUID(ssp); b != "" {
			claims[b] = append(claims[b], ssp.UUID)
		}
	}
	ids = map[string]string{}
	contested = map[string][]string{}
	for _, ssp := range ssps {
		if ssp == nil {
			continue
		}
		b := sparcBoundaryUUID(ssp)
		switch {
		case b == "":
			ids[ssp.UUID] = ssp.UUID
		case len(claims[b]) > 1:
			ids[ssp.UUID] = ssp.UUID
			contested[b] = claims[b]
		default:
			ids[ssp.UUID] = b
		}
	}
	return ids, contested
}
