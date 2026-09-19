# 03 Data model

Each tier has exactly one OSCAL home. Anything OSCAL does not model goes in a single namespace. UUIDs are derived from natural keys, and evidence always lives in back-matter with a hash.

OSCAL has no native organization or federation hierarchy, so those tiers are expressed through `party` objects and SPARC's trust fabric rather than by stretching an SSP to fit.

## Where each tier lives

| Tier | OSCAL home | Join key | Roles from |
|---|---|---|---|
| Federation | `party` (organization) plus SPARC's signed federation manifest | Federation party UUID | Manifest signers |
| Organization | `party` with `member-of-organizations`; common controls in a common-control-provider SSP | Org party UUID, reused in every boundary SSP | AO in `responsible-parties` |
| Boundary | One SSP per authorization boundary, with AR and POA&M chained to it | `system-id` plus SSP UUID | SO and ISO in `responsible-parties` |
| System | SSP `components` and `inventory-items` | Inventory `asset-id` matched to the HDF target | Component `responsible-roles` |

Because the same organization party UUID appears in every SSP beneath it, the tree is built by joining on UUIDs. No separate hierarchy database is needed.

## Namespace contract

Defined in [`schemas/sparc-namespace-props.v1.schema.json`](../schemas/sparc-namespace-props.v1.schema.json) and enforced by `sparc-validate`. Changes within `v1` are additive only.

| Prop | On | Example | Drives |
|---|---|---|---|
| `node-type` | Party, SSP metadata | `boundary` | Tree builder |
| `parent-uuid` | SSP metadata | Org party UUID | Tree builder, rollups |
| `next-decision-date` | SSP metadata | `2026-10-11` | Snap to decision, AO horizon |
| `fips-199` | SSP metadata | `high` | Rollup weight |
| `blocks-ato` | Risk | `true` | Red cells, ranking |
| `evidence-kind` | Back-matter resource | `manual-attestation` | Chain view, decay rules |
| `signed-by` | Back-matter resource | ISO party UUID | Signature verification |
| `condition-expires` | Risk | `2026-11-01` | AO decision reopen |
| `trigger` | Risk | `score<0.85` | AO decision reopen |

## Boundary SSP metadata

Roles come from the documents, not an admin screen. When an AO changes, the SSP changes, and access follows.

```json
"metadata": {
  "props": [
    {"name": "node-type", "ns": "https://risk-sentinel.org/ns/sparc", "value": "boundary"},
    {"name": "parent-uuid", "ns": "https://risk-sentinel.org/ns/sparc", "value": "<org-party-uuid>"},
    {"name": "next-decision-date", "ns": "https://risk-sentinel.org/ns/sparc", "value": "2026-10-11"},
    {"name": "fips-199", "ns": "https://risk-sentinel.org/ns/sparc", "value": "moderate"}
  ],
  "responsible-parties": [
    {"role-id": "authorizing-official", "party-uuids": ["<ao-uuid>"]},
    {"role-id": "system-owner", "party-uuids": ["<so-uuid>"]},
    {"role-id": "information-system-security-officer", "party-uuids": ["<iso-uuid>"]}
  ]
}
```

## Evidence chain

Back-matter resource → observation → finding → risk → POA&M item. The observation's native `expires` field drives decay and countdowns in the HUD.

```json
{
  "uuid": "<uuid5>",
  "title": "CP-4 tabletop report FY26",
  "props": [
    {"name": "evidence-kind", "ns": "https://risk-sentinel.org/ns/sparc", "value": "manual-attestation"},
    {"name": "signed-by", "ns": "https://risk-sentinel.org/ns/sparc", "value": "<iso-uuid>"}
  ],
  "rlinks": [
    {"href": "s3://evidence/portal/cp-4/2026-09.pdf", "media-type": "application/pdf",
     "hashes": [{"algorithm": "SHA-256", "value": "<sha256>"}]},
    {"href": "s3://evidence/portal/cp-4/2026-09.pdf.sig", "media-type": "application/pkcs7-signature"}
  ]
}
```

```json
{
  "uuid": "<uuid5>",
  "methods": ["EXAMINE"],
  "types": ["control-objective"],
  "collected": "2026-09-12T00:00:00Z",
  "expires": "2027-09-12T00:00:00Z",
  "relevant-evidence": [{"href": "#<resource-uuid>"}]
}
```

- The ISO's sign-off is recorded in the assessment result's `attestations`.
- Automated results keep the original HDF file as a hashed back-matter resource, so every finding traces to its raw scan.
- Observations bind to components by matching the HDF target to the inventory `asset-id`.

## Deterministic UUIDs

```go
var NS = uuid.MustParse("<federation namespace uuid>") // registered once

func Key(parts ...string) uuid.UUID {
    return uuid.NewSHA1(NS, []byte(strings.Join(parts, "|")))
}

// Key(boundarySystemID, "cp-4", componentUUID, "attestation", "2026-Q3")
```

Object UUIDs stay stable across reruns and instances, so federated peers deduplicate without coordinating. Only document-level UUIDs change per revision, as the OSCAL spec intends.

## Inheritance and hybrid controls

- A provider's `export` declares `provided` statements and `responsibilities`.
- Consumers answer with `inherited` and `satisfied` statements.
- A hybrid control carries two responsibility UUIDs and is green only when both halves have unexpired observations.
- `leveraged-authorizations` points to a back-matter resource whose link is the provider's federated SPARC endpoint plus a hash. That link feeds the reverse-inheritance index behind blast radius.
