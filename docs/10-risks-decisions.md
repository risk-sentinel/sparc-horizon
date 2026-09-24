# 10 Risks and decisions

## Risks

| Risk | Mitigation |
|---|---|
| ~~The federation namespace is unregistered, so every derived identifier is provisional~~ — **registered 2026-09-21** (`sparc#1155`) as `9f434272-f796-589b-b972-954790395630`, derived from `https://sparc.risk-sentinel.org/ns` | Retired. Adopted in #54: Horizon took SPARC's registered URI in place of its own placeholder, and the fixtures regenerated. Identifiers are final |
| ~~**Horizon and SPARC disagree about `family-id` normalisation.**~~ **RESOLVED 2026-09-23.** The shared contract lowercased it unconditionally; Horizon carried a foreign-vocabulary family as issued, so the same input derived two cell identifiers with neither side erroring | **Settled in Horizon's favour.** [`sparc#1175`](https://github.com/risk-sentinel/sparc/issues/1175) merged as `1cb999b1`: `vocabulary-normalisers` is keyed by vocabulary **and** identifier kind, and two new vectors carry exactly the identifiers Horizon already derived. No UUID moved. Re-vendored in #68; the restriction in `docs/03-data-model.md` is lifted. The test that pinned the divergence now asserts agreement |
| Party UUIDs drift between documents and break the tree | A validate rule plus a tree builder that reports orphans instead of silently dropping them |
| ~~`go-oscal` lags OSCAL 1.2.x~~ — **not true as of v0.7.1.** It ships `oscal-1-2-1` and `oscal-1-2-2` type packages | Retired. Decided in P0 (#26): adopt `go-oscal`, pinned. The two risks that replaced it are the next rows |
| **`go-oscal` collapses four OSCAL assemblies into one Go type.** OSCAL defines `local-definitions` four times with four different shapes and gives three of them the same schema title, "Local Definitions". `go-oscal` names generated types from that title, so the three become one struct carrying the **assessment-plan** shape. Consequences run both ways: **reading**, a schema-valid assessment-results document loses `results[*].local-definitions.tasks` **and** `.assessment-assets`; **writing**, the same struct accepts `components`, `inventory-items` and `users` on an assessment-results document, where OSCAL forbids them. Identical in the OSCAL 1.2.3 schema and in `go-oscal`'s unreleased 1.2.3 types, so no version bump resolves it | Both directions are pinned by tests over an authored, schema-valid document, and the corpus baseline in `internal/oscal/testdata/measurements.json` fails when the measurement changes. Horizon does not project from result tasks or assessment assets today, and the rule from #26 — hash and verify **received bytes**, never a re-serialisation — keeps the reading half out of anything signed. Two standing instructions follow: **do not re-emit a parsed assessment-results document as if it were the original**, and **schema-validate anything Horizon emits**, because the type system will not catch the writing half. **Unreported upstream as of 2026-09-21; the report is written and waiting on the owner** — [`docs/dev/go-oscal-local-definitions.md`](dev/go-oscal-local-definitions.md), with a standalone reproducer |
| Crosswalk quality skews the KSI axis and ranking reach | Show the mapping source on every swapped cell; flag low-confidence mappings |
| The HUD ranks the wrong thing first and loses trust | Configurable weights, logged rankings, and a user session every two weeks |
| PKI is not available in the prototype environment | A dev CA behind the same signing interface |
| The peer federation API isn't ready | Exchange signed bundles as files first |
| A hostile peer submits a document under an object UUID it does not own | UUIDv5 over natural keys is deterministic and its grammar is published, so any peer can compute any boundary's identifiers. Deduplicate on object UUID **and** originating party; treat a clash as a conflict to surface, not a duplicate to collapse. Threat model TM-6 |
| Simulated state escapes the what-if overlay | Isolation is structural, not a runtime guard: the writer and emitter take a handle an overlay view cannot produce. Threat model TM-2 |
| A signature fails on a document nobody tampered with | Typed OSCAL normalises timestamps (`+00:00` becomes `Z`) — same instant, different bytes. JCS canonicalises member order and numbers, **not** string values, so a parse then re-serialise cycle changes the digest. Verify and hash over the **received bytes**, never a re-serialisation. Measured in #26 |
| A signing key is compromised and forged attestations recompute correctly | Sign through the platform key service so the key is never in the service's memory; record `signed-by` so revocation identifies the affected window; answer the blast radius with a what-if overlay. Threat model TM-5 |

## Decisions for phase 0

- Signature format: CMS detached, a Sigstore bundle, or JWS over canonical JSON. Whichever is chosen, it signs the bytes as received — see the round-trip row above
- **How an OIDC subject maps to a party UUID is unspecified.**
  [`docs/07-security.md`](07-security.md) asserts "Party UUIDs map to subjects, so roles come from
  `responsible-parties` in the documents", and nothing says by what mechanism. Found in #76, which
  needed it: the fixture parties carry **no `external-ids`, no props and no email addresses**, and
  none of the nine namespace props covers it. Three candidates, none free:
  **(a)** OSCAL's native `party.external-ids` (`PartyExternalIdentifier`, scheme + id), which is
  where OSCAL already puts an external identifier and so needs no namespace change;
  **(b)** a tenth namespace prop, which is a v1 change and must be additive — the `enum` and a
  matching `allOf` branch — and puts identity in Horizon's namespace rather than OSCAL's;
  **(c)** a mapping held by SPARC and fetched, which keeps identity out of the documents and makes
  authorization depend on a service call rather than on an export, weakening the recompute property.
  Until it is settled **`internal/authz` takes a party UUID as the caller**, so the decision can be
  made without rewriting the decision logic
- **Sibling order in a `Node`'s `children` is not specified by the contract, and the P0 goldens
  encode a generator artifact.** `api/openapi.yaml` orders things explicitly where it means to —
  heatmap columns are "ordered by blockers descending" — and `Node.children` is a plain array.
  The persona goldens in `fixtures/api/` carry the order of the `Boundaries` table inside
  `internal/fixtures`, which **no OSCAL document determines**: nothing says whether Public Portal
  precedes Identity Services. `internal/tree` therefore sorts the tiers assembled across documents
  and preserves declaration order at the system tier, where the children come from one document and
  the order is the SSP author's. #76's comparison against the goldens is order-insensitive among
  siblings for that reason. **Either the contract should state an order or it should say the order
  is unspecified**; leaving it implicit means a client may come to depend on the generator's table
- **A party bound at two disjoint nodes has no single tree to be shown.** `/v1/tree` returns one
  `Node`, and the caller's tree is rooted at the node they hold a role on. Nothing in the fixtures
  exercises a party holding roles in two organizations, and the contract has no shape for it. #76
  takes the conservative direction — the highest binding roots the tree and the others are
  unreachable, showing too little rather than too much — but that silently withholds access
  somebody was granted, so it is a placeholder rather than an answer
- ~~`go-oscal` versus types generated from the NIST JSON schemas~~ — **decided 2026-09-19 in #26:**
  `go-oscal`, pinned. Its 1.2.x coverage is real, the round trip is lossless apart from timestamp
  normalisation, and the "generated types" fallback turned out to be the same generator
  self-hosted, for no fidelity gain. **Re-tested 2026-09-21 in #49** against eight published NIST
  documents covering all seven models — including the SSP, profile and POA&M the original spike
  could not exercise. **The decision stands, with one measured limitation** (the risk table
  above): seven of eight documents are lossless at their declared version and strict-decode
  clean, and the eighth loses two fields from `results[*].local-definitions`. Cross-version
  decoding was measured too: every document in the corpus decodes identically under all four
  supported type packages, so the gap is a collapsed assembly rather than a version-skew problem
- **The FIPS 199 rollup weights have no values.** `docs/05-projection-engine.md` weights totals by
  impact level and never says by how much, while the ranking weights it does discuss are
  explicitly configuration. `internal/project.FIPSWeight` carries a documented default (3, 2, 1)
  so the mock's responses are reproducible — it is not a decision. The weighting changes which
  boundary a person is told to look at first, so P2 settles it with users
- Who owns the ranking weights: each AO, or the organization
- When the ledger moves from SQLite to Postgres
- Whether decision dates live only in SSP metadata or also come from the GRC calendar
- **Where the ledger's chain head is anchored.** The chain detects truncation and edits of
  an export, but the head is published by the party that holds the ledger, so a wholesale
  rewrite can be republished consistently. The head needs an anchor its writer does not
  control: emitted to the evidence bucket under the existing role-scoped prefix,
  countersigned through the SPARC federation, or both. This constrains the federation
  contract, so it is a phase-0 decision rather than an implementation detail.
  Threat model TM-4
