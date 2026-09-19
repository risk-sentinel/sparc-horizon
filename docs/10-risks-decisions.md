# 10 Risks and decisions

## Risks

| Risk | Mitigation |
|---|---|
| Party UUIDs drift between documents and break the tree | A validate rule plus a tree builder that reports orphans instead of silently dropping them |
| ~~`go-oscal` lags OSCAL 1.2.x~~ — **not true as of v0.7.1.** It ships `oscal-1-2-1` and `oscal-1-2-2` type packages, and all five real documents tested round-trip with no unknown fields | Retired. Decided in P0 (#26): adopt `go-oscal`, pinned. The risk that replaced it is the next row |
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
  self-hosted, for no fidelity gain. Re-test against the P0 fixture federation before P1 builds
  `internal/oscal`: SSP, profile and POA&M have no fixtures yet and so were not exercised
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
