# 10 Risks and decisions

## Risks

| Risk | Mitigation |
|---|---|
| Party UUIDs drift between documents and break the tree | A validate rule plus a tree builder that reports orphans instead of silently dropping them |
| `go-oscal` lags OSCAL 1.2.x | Decide in phase 0; generated types are the fallback |
| Crosswalk quality skews the KSI axis and ranking reach | Show the mapping source on every swapped cell; flag low-confidence mappings |
| The HUD ranks the wrong thing first and loses trust | Configurable weights, logged rankings, and a user session every two weeks |
| PKI is not available in the prototype environment | A dev CA behind the same signing interface |
| The peer federation API isn't ready | Exchange signed bundles as files first |

## Decisions for phase 0

- Signature format: CMS detached, a Sigstore bundle, or JWS over canonical JSON
- Who owns the ranking weights: each AO, or the organization
- When the ledger moves from SQLite to Postgres
- Whether decision dates live only in SSP metadata or also come from the GRC calendar
