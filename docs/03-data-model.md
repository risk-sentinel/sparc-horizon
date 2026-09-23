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

The namespace is **`https://sparc.risk-sentinel.org/ns`**, registered by SPARC and adopted here
in #54. Horizon carried a placeholder of its own until `sparc#1155` settled it; a namespace
identifies an authority's vocabulary, so there is one string and SPARC owns it.

**The schema is a selective validator, not a whole-document one.** It applies to props whose
`ns` is `https://sparc.risk-sentinel.org/ns`, and to every one of them. Applying it to every
prop regardless of `ns` rejects valid OSCAL, because `ns` is **optional** in OSCAL and an absent
one means the default NIST namespace rather than this one. `ns` stays required *within* the
schema for exactly that reason: it is what stops a prop claiming one of these nine names from
outside the namespace.

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

### Horizon owns one namespace and reads several

A real document carries props from more than one authority. These are the namespaces observed
in content this prototype is built against, recorded so the rule below is written from evidence
rather than assumption:

| Namespace | Seen in | Note |
|---|---|---|
| `https://sparc.risk-sentinel.org/ns` | This estate | The nine props above. Closed enum, additive only. Registered in `sparc#1155` |
| `http://aws.amazon.com/ns/oscal` | AWS Labs' catalog and its 231 service component definitions | 1809 props across five names. **CamelCase** — `SeverityLabel`, `TriggerType`, `EvaluatedServices`, `TechnicalControlId` |
| *absent* | The same component definitions | OSCAL makes `ns` **optional**; an absent one means the default NIST namespace |
| `http://fedramp.gov/ns/oscal` | FedRAMP extensions | |
| `http://csrc.nist.gov/ns/oscal` | The OSCAL default | |

DISA and CIS publish XCCDF **XML** namespaces. Which prop `ns` their content lands on after
conversion is decided by the converter, which is `sparc-validate`'s, not settled here.

**Pass-through preservation is a correctness requirement, not a courtesy.** A prop in a
namespace Horizon does not own is **preserved as issued** — name, namespace and value, with no
normalisation of any of them, including the CamelCase — and re-emitted unchanged. The recompute
audit requires a cell to recompute from exported OSCAL alone, so a foreign prop dropped on
ingest breaks that audit for everyone downstream, not just here.

Horizon validates only its own namespace and interprets only its own props. It never rejects a
document for carrying someone else's.

Organization-defined parameters do **not** need a namespace at all: OSCAL models them natively
as `parameters` with `set-parameters`, and roles come from `responsible-parties`. Where OSCAL
has somewhere to put something, it goes there.

## Boundary SSP metadata

Roles come from the documents, not an admin screen. When an AO changes, the SSP changes, and access follows.

```json
"metadata": {
  "props": [
    {"name": "node-type", "ns": "https://sparc.risk-sentinel.org/ns", "value": "boundary"},
    {"name": "parent-uuid", "ns": "https://sparc.risk-sentinel.org/ns", "value": "<org-party-uuid>"},
    {"name": "next-decision-date", "ns": "https://sparc.risk-sentinel.org/ns", "value": "2026-10-11"},
    {"name": "fips-199", "ns": "https://sparc.risk-sentinel.org/ns", "value": "moderate"}
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
    {"name": "evidence-kind", "ns": "https://sparc.risk-sentinel.org/ns", "value": "manual-attestation"},
    {"name": "signed-by", "ns": "https://sparc.risk-sentinel.org/ns", "value": "<iso-uuid>"}
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

**The federation namespace is registered** (`sparc#1155`, 2026-09-21), and derived rather than
invented so every implementation reaches the same value from the same published string instead
of copying a constant:

```
namespace = uuidv5(url-namespace, "https://sparc.risk-sentinel.org/ns")
          = 9f434272-f796-589b-b972-954790395630
```

`keys.Namespace()` recomputes it. Any peer can do the same and verify it provably belongs to
that URI. The identifiers in `fixtures/` derive under it and are final.

**The separator is `\x1f` (ASCII unit separator), not `|`.** A printable delimiter is
ambiguous: with `|`, `("a|b", "c")` and `("a", "b|c")` produce the same input and therefore the
same UUID — a collision by accident, which is harder to notice than one by attack. `\x1f`
cannot appear in any field value defined below, and an implementation **must reject** a field
containing it rather than escape it, so the ambiguity is impossible rather than merely unlikely.

**The grammar version is part of the input.** A change to any field list changes every
identifier derived under it, which is as breaking as changing the namespace. Bumping `grammar`
makes that visible and keeps old and new identifiers from being mistaken for each other. A
grammar change is a v2 of this document, never an edit to v1.

> **`source-uuid` was added to v1 rather than producing a v2** (#37), and **the namespace URI
> changed within v1 for the same reason** (#54). The rule exists so old and new identifiers
> cannot be mistaken for one another, and both times it was checked rather than waived: nothing
> durable derived from the earlier form. When #37 landed, no implementation of these field lists
> existed anywhere. When #54 landed, the only identifiers deriving from them were this
> repository's own fixtures — explicitly provisional pending the registration, and regenerated
> by that change — while `sparc-validate`'s rules were unwritten and `sparc` derived no live
> identifiers.
>
> These are exceptions justified by verifiable facts, not a precedent. Now that the namespace is
> registered and the fixtures are final, any further change is a v2.

### Canonical forms

Two implementations must produce identical bytes for the same logical key, so every field is
normalised before it is joined.

| Field kind | Canonical form |
|---|---|
| Control id | **Within the NIST SP 800-53 vocabulary only:** OSCAL's lowercase dotted form, `ac-2.1`, never `AC-2(1)`. Enhancements use `.`, not parentheses, and nest — the canonical value matches `^[a-z]{2,3}-\d+(\.\d+)*$`. Resolve through SPARC's mapping documents before deriving. A control identifier from **any other catalog is carried exactly as that catalog issues it** — see below |
| Family id | **Within the NIST vocabulary:** lowercase, two or three letters. From any other authority, carried as issued — see the disagreement noted below |
| UUID | Lowercase hex with hyphens, RFC 9562 §4 |
| Period | `YYYY-Qn` \| `YYYY-MM`, zero-padded. Quarters are `2026-Q3`, never `2026Q3` or `Q3-2026`. **Narrowed in #54** to SPARC's shared contract, which carries neither a bare year nor a full date |
| Decision date | `YYYY-MM-DD`. An AO decision is keyed on the day it was taken, so the value lines up with the `next-decision-date` prop the HUD counts down to |
| Object-kind token | Fixed lowercase ASCII from the table below. Not free text |
| Any other string | Unicode **NFC**, no trimming, no case folding — if a field needs case folding to match, it is the wrong field |

**Canonicalisation is valid only within a vocabulary.** Lowercasing the AWS Security Hub
identifier `ACM.1` yields `acm.1`, which looks like a NIST control and is not one. Normalising
another authority's identifier into a form it never issued produces a value that still
validates, still derives a UUID, and names nothing — worse than rejecting it, because nothing
downstream can tell.

So the NIST rules above apply when `source-uuid` resolves to an 800-53 catalog, or to a profile
over one. Everywhere else the identifier is NFC-normalised as an ordinary string and otherwise
left alone. **`source-uuid` is what makes that decidable:** without it there is no way to know
which vocabulary an identifier came from.

**Within the NIST vocabulary, these spellings are accepted and converge.** SPARC's own API
emits more than one of them (`sparc#1162`), so an implementation that assumes identifiers
arrive canonical mints two identifiers for one control.

| Accepted | Canonical |
|---|---|
| `AC-2`, `ac-2`, `AC-02`, `Ac-2` | `ac-2` |
| `AC-2(1)`, `AC-2 (1)`, `AC-2.1`, `AC-02(01)`, `ac-2.1` | `ac-2.1` |
| `AC`, `ac` — a family, for a projection cell column | `ac` |

Anything else is **rejected, not repaired**: a statement fragment (`ac-2_smt.a`) names part of a
control rather than a control, a third level (`ac-2.1.3`) names nothing, and a control numbered
zero does not exist. Rejection is the point — a repaired identifier still validates, still
derives a UUID, and may name the wrong object.

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
| Attestation | `parent-ssp-uuid`, `"attestation"`, `source-uuid`, `control-id`, `component-uuid`, `period` |
| Observation | `parent-ssp-uuid`, `"observation"`, `source-uuid`, `control-id`, `component-uuid`, `period` |
| Finding | `parent-ssp-uuid`, `"finding"`, `source-uuid`, `control-id`, `component-uuid`, `period` |
| Risk | `parent-ssp-uuid`, `"risk"`, `source-uuid`, `control-id`, `component-uuid`, `period` |
| POA&M item | `parent-ssp-uuid`, `"poam-item"`, `source-uuid`, `control-id`, `component-uuid`, `period` |
| Evidence resource | `parent-ssp-uuid`, `"resource"`, `sha256-of-content` |
| AO decision | `parent-ssp-uuid`, `"decision"`, `risk-uuid`, `period` |
| Responsibility half | `parent-ssp-uuid`, `"responsibility"`, `source-uuid`, `control-id`, `component-uuid`, `"provider"` \| `"consumer"` |
| Projection cell | `node-uuid`, `"cell"`, `source-uuid`, `control-id` \| `family-id`, `horizon-bucket` |

An evidence resource is keyed by the **hash of its content**, not by a period: the same bytes
submitted twice are the same resource, and that is what a back-matter hash already claims. It
takes no `source-uuid`, because bytes are not scoped to a catalog. Neither does an AO decision,
which is keyed on the risk it decides.

### `source-uuid` — the catalog a control identifier belongs to

**A control identifier is unique only within the catalog that defines it.** `ac-2.1` is a NIST
SP 800-53 control; `ACM.1` is an AWS Security Hub control; a CIS benchmark numbers its own from
`1.1.1`. One SSP can carry several of these at once — Security Hub identifiers arrive through
inherited AWS service components — and `parent-ssp-uuid` does not separate them, because they
are in the same SSP.

Without a qualifier the key space does not partition by authority. That is the same defect as
an ambiguous delimiter, in a different field: not that a collision is likely, but that nothing
makes it **impossible**.

**`source-uuid` is the UUID of the catalog or profile that defines the control**, resolved as:

1. The `source` on the `control-implementation` that declares the requirement. It is a
   document-local `#fragment`, so it is **resolved to the back-matter resource it names, and the
   resource's UUID is used** — a fragment is local to one document and would not federate.
2. **Where `source` is absent, the SSP's `import-profile`**, resolved the same way. Component
   definitions carry `source` per control-implementation because they are component-scoped; the
   profile is what holds the resolved control set and the organization-defined parameters and
   statements for the system.

An implementation **must reject** a control-carrying object it cannot resolve a `source-uuid`
for, rather than deriving without one. A key missing a field is not a key with an empty field —
it is a different key, and two implementations disagreeing about that is the silent divergence
this grammar exists to prevent.

**The resource that names an external catalog or profile uses that document's own UUID.**
Back-matter resource UUIDs are otherwise arbitrary, and a fresh one per citing document would
give the same catalog a different `source-uuid` in every SSP that referenced it — which is
precisely the federation-wide deduplication the qualifier exists to make possible. The fixtures
follow this convention; it was not stated before generating them, and generating them is how it
surfaced.

A UUID is used rather than the catalog's own identifier string because it is already unique,
already stable for the life of the document, and already how everything else here joins.

Projection cells are materialised, not exchanged, so their identifiers are local. They use the
same grammar anyway, because an identifier scheme with an exception is an identifier scheme
someone will use inconsistently.

### Worked example

```
namespace = <federation namespace uuid>
fields    = ["3fa85f64-5717-4562-b3fc-2c963f66afa6",   // parent ssp
             "attestation",
             "b7e21d90-4c1a-4f55-9e33-0a6d2c118f44",   // source-uuid: the
                                                       // 800-53 catalog, from
                                                       // back-matter
             "cp-4",                                   // canonical: NIST vocab
             "9f1c…",                                  // component
             "2026-Q3"]
input     = "v1\x1f3fa85f64-…\x1fattestation\x1fb7e21d90-…\x1fcp-4\x1f9f1c…\x1f2026-Q3"
uuid      = uuidv5(namespace, input)
```

**Why the qualifier is load-bearing.** The same component, period and SSP, assessed against two
different catalogs, must not collide:

```
800-53:       … \x1fb7e21d90-…\x1fcp-4\x1f9f1c…\x1f2026-Q3     -> one uuid
Security Hub: … \x1f5d40a72c-…\x1fACM.1\x1f9f1c…\x1f2026-Q3    -> a different uuid
```

Note the second control identifier is **not** lowercased. It is not a NIST control, and `acm.1`
would name nothing in any catalog.

Regenerating from the same source documents yields the same UUID, which is the property the
P0 exit criterion measures: identical UUIDs across two independent regenerations.

### Reference implementation and the shared contract

The Go reference implementation is [`internal/keys`](../internal/keys), over the field
normalisation in [`internal/canonical`](../internal/canonical). It has no exported way to derive
from raw strings: every entry point canonicalises its own fields and rejects what it cannot,
because an identifier minted from a value another implementation would have normalised
differently is exactly the divergence this grammar exists to prevent.

**The vectors are no longer published from here.** `sparc#1161` made
`sparc:lib/federation/key-grammar.v1.json` the source of truth — the field lists, the type
rules, the vectors and the dedup rule, in one file the Ruby, Python and Go implementations all
assert against. Horizon vendors it into `internal/keys/testdata/` with its provenance and
conforms to it. The file began life here, as `fixtures/key-vectors.v1.json`, and was regenerated
upstream under the registered namespace.

**Conformance is checked on more than the vectors.** Every vector agreed on the day Horizon
adopted the contract, and reading its *type rules* still found five disagreements — none of
which any vector exercised. The test therefore drives the contract's own regexes against this
implementation's canonicalisers, rather than comparing a fixed corpus of UUIDs.

**One rule was not adopted; it is now settled, in Horizon's favour.** The contract used to
normalise `family-id` by lowercasing it unconditionally. Under a non-NIST vocabulary that is the
defect this document removed from control identifiers: an AWS Security Hub family is `ACM`, and
`acm` names nothing. Horizon carried a foreign family as issued, so the two implementations
derived **different identifiers for the same projection cell** — `ACM` under an opaque source
gave `b9691843-…` here and `f2a38fbf-…` there, with neither side erroring.

`sparc#1175` fixed it upstream (`1cb999b1`): `vocabulary-normalisers` is keyed by vocabulary
**and** identifier kind, so an opaque family is carried as issued, and the contract gained two
vectors holding exactly the identifiers Horizon already derived. **No UUID moved.** Horizon
re-vendored at that commit in #68, and the restriction that followed from the disagreement —
do not derive a cell identifier for a foreign-vocabulary family against a peer — is **lifted**.
The write-up, with the resolution, is
[`docs/dev/sparc-family-id-normalisation.md`](dev/sparc-family-id-normalisation.md).

### Status

The federation namespace UUID **is registered** — `9f434272-f796-589b-b972-954790395630`
(`sparc#1155`, adopted here in #54). Identifiers derived under it are final, and the fixtures
that were built against the earlier placeholder have been regenerated. The grammar itself never
depended on that registration and is fixed by this document.

## Inheritance and hybrid controls

- A provider's `export` declares `provided` statements and `responsibilities`.
- Consumers answer with `inherited` and `satisfied` statements.
- A hybrid control carries two responsibility UUIDs and is green only when both halves have unexpired observations.
- `leveraged-authorizations` points to a back-matter resource whose link is the provider's federated SPARC endpoint plus a hash. That link feeds the reverse-inheritance index behind blast radius.
