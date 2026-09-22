# 10 Risks and decisions

## Risks

| Risk | Mitigation |
|---|---|
| ~~The federation namespace is unregistered, so every derived identifier is provisional~~ — **registered 2026-09-21** (`sparc#1155`) as `9f434272-f796-589b-b972-954790395630`, derived from `https://sparc.risk-sentinel.org/ns` | Retired. Adopted in #54: Horizon took SPARC's registered URI in place of its own placeholder, and the fixtures regenerated. Identifiers are final |
| **Horizon and SPARC disagree about `family-id` normalisation.** The shared contract lowercases it unconditionally; Horizon carries a foreign-vocabulary family as issued. The same input therefore derives two cell identifiers, and neither side errors | The divergence is pinned by a test rather than tolerated, and `docs/03-data-model.md` says not to derive a cell identifier for a foreign-vocabulary family against a peer until it is settled. **Filed upstream 2026-09-22 as [`sparc#1175`](https://github.com/risk-sentinel/sparc/issues/1175)**, from [`docs/dev/sparc-family-id-normalisation.md`](dev/sparc-family-id-normalisation.md) |
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
