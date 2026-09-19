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

Object UUIDs are UUIDv5 over a natural key, so they are stable across reruns and instances and
federated peers deduplicate without coordinating. Only document-level UUIDs change per
revision, as the OSCAL spec intends — a revision is a different document.

This section is normative. Every phase joins on these identifiers, and reference
implementations are published in more than one runtime, so a disagreement between two
implementations is a silent data-integrity fault rather than a bug someone notices.

### What a key identifies

**An object UUID identifies a thing, not an assertion about a thing.** The asserting party is
carried alongside, in `responsible-parties`, an observation's `origins`, or the `signed-by`
prop — never folded into the key.

This is load-bearing in both directions:

- Fold the party in, and two peers attesting the same control period derive two different
  identifiers. Deduplication fails completely, and the federation sees duplicates everywhere.
- Leave it out, as specified here, and any peer can compute any boundary's identifiers, because
  the namespace is shared and this grammar is public. So a consumer must deduplicate on
  **(object UUID, originating party)** and treat a clash between two parties as a conflict to
  surface, never a duplicate to collapse. See threat model TM-6.

The grammar makes the identifier predictable on purpose. Predictable is not the same as
trustworthy, and the consumer is where that difference is enforced.

### Derivation

```
namespace  = <federation namespace uuid>      // registered once, estate-wide
grammar    = "v1"                             // this document's version
uuid(obj)  = uuidv5(namespace, grammar + "\x1f" + join(fields(obj), "\x1f"))
```

`uuidv5` is SHA-1 based (`uuid.NewSHA1` in Go), per RFC 9562 §5.5.

**The separator is `\x1f` (ASCII unit separator), not `|`.** A printable delimiter is
ambiguous: with `|`, `("a|b", "c")` and `("a", "b|c")` produce the same input and therefore the
same UUID — a collision by accident, which is harder to notice than one by attack. `\x1f`
cannot appear in any field value defined below, and an implementation **must reject** a field
containing it rather than escape it, so the ambiguity is impossible rather than merely unlikely.

**The grammar version is part of the input.** A change to any field list changes every
identifier derived under it, which is as breaking as changing the namespace. Bumping `grammar`
makes that visible and keeps old and new identifiers from being mistaken for each other. A
grammar change is a v2 of this document, never an edit to v1.

### Canonical forms

Two implementations must produce identical bytes for the same logical key, so every field is
normalised before it is joined.

| Field kind | Canonical form |
|---|---|
| Control id | OSCAL's lowercase dotted form: `ac-2.1`, never `AC-2(1)`. Enhancements use `.`, not parentheses. Resolve through SPARC's mapping documents before deriving |
| UUID | Lowercase hex with hyphens, RFC 9562 §4 |
| Period | `YYYY` \| `YYYY-Qn` \| `YYYY-MM` \| `YYYY-MM-DD`, zero-padded. Quarters are `2026-Q3`, never `2026Q3` or `Q3-2026` |
| Object-kind token | Fixed lowercase ASCII from the table below. Not free text |
| Any other string | Unicode **NFC**, no trimming, no case folding — if a field needs case folding to match, it is the wrong field |

### What may appear in a natural key

Only values that cannot change without the object becoming a **different object**.

Permitted: UUIDs of parents and referents, control ids, the object-kind token, period labels,
and the fixed enumerations in this document.

**Excluded, always:** titles, descriptions, prose, statuses, scores, timestamps other than a
period label, file paths, URLs, and anything a person edits for clarity. A title in a key means
correcting a typo mints a new object and orphans the old one.

### Field lists per object type

| Object | Fields, in order |
|---|---|
| Attestation | `parent-ssp-uuid`, `"attestation"`, `control-id`, `component-uuid`, `period` |
| Observation | `parent-ssp-uuid`, `"observation"`, `control-id`, `component-uuid`, `period` |
| Finding | `parent-ssp-uuid`, `"finding"`, `control-id`, `component-uuid`, `period` |
| Risk | `parent-ssp-uuid`, `"risk"`, `control-id`, `component-uuid`, `period` |
| POA&M item | `parent-ssp-uuid`, `"poam-item"`, `control-id`, `component-uuid`, `period` |
| Evidence resource | `parent-ssp-uuid`, `"resource"`, `sha256-of-content` |
| AO decision | `parent-ssp-uuid`, `"decision"`, `risk-uuid`, `period` |
| Responsibility half | `parent-ssp-uuid`, `"responsibility"`, `control-id`, `component-uuid`, `"provider"` \| `"consumer"` |
| Projection cell | `node-uuid`, `"cell"`, `control-id` \| `family-id`, `horizon-bucket` |

An evidence resource is keyed by the **hash of its content**, not by a period: the same bytes
submitted twice are the same resource, and that is what a back-matter hash already claims.

Projection cells are materialised, not exchanged, so their identifiers are local. They use the
same grammar anyway, because an identifier scheme with an exception is an identifier scheme
someone will use inconsistently.

### Worked example

```
namespace = <federation namespace uuid>
fields    = ["3fa85f64-5717-4562-b3fc-2c963f66afa6", "attestation",
             "cp-4", "9f1c…", "2026-Q3"]
input     = "v1\x1f3fa85f64-…\x1fattestation\x1fcp-4\x1f9f1c…\x1f2026-Q3"
uuid      = uuidv5(namespace, input)
```

Regenerating from the same source documents yields the same UUID, which is the property the
P0 exit criterion measures: identical UUIDs across two independent regenerations.

### Status

The federation namespace UUID is **not yet registered** — `sparc#1155`. Until it is, identifiers
derived under the placeholder are provisional and fixtures built from them must be regenerated.
The grammar itself does not depend on that registration and is fixed by this document.

## Inheritance and hybrid controls

- A provider's `export` declares `provided` statements and `responsibilities`.
- Consumers answer with `inherited` and `satisfied` statements.
- A hybrid control carries two responsibility UUIDs and is green only when both halves have unexpired observations.
- `leveraged-authorizations` points to a back-matter resource whose link is the provider's federated SPARC endpoint plus a hash. That link feeds the reverse-inheritance index behind blast radius.
