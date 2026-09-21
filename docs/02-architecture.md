# 02 Architecture

One Go binary with an embedded TypeScript UI, shipped as a distroless container. Results and mappings flow up; attestations and decisions flow down.

```mermaid
flowchart TB
  subgraph Lenses[Role lenses]
    AO[AO lens<br/>portfolio, decisions, conditions]
    SO[SO lens<br/>boundary fix list, POA&M]
    ISO[ISO lens<br/>attest, review evidence]
  end
  subgraph Horizon[Horizon - Go]
    TREE[Tree builder]
    LEDGER[Ledger<br/>append-only, hash-chained]
    PROJ[Projection engine]
    ACT[Attest and decide]
  end
  subgraph SPARC
    AUTH[Authoritative<br/>catalogs, baselines, profiles]
    VAL[Validation<br/>sparc-validate, namespace rules]
    DEL[Delivery<br/>crosswalks, federation trust]
  end
  subgraph Pipelines
    GL[Reusable GitLab include<br/>SAST, SCA, IaC, container]
    RT[Runtime scans<br/>AWS Config, InSpec]
  end
  Lenses <-->|HTTPS, OIDC, node-scoped roles| Horizon
  Horizon <-->|mTLS: SSP, AR, POA&M, mappings / attestations, decisions| SPARC
  Pipelines -->|HDF v3 via HDF CLI| SPARC
```

## Components

| Component | Responsibility |
|---|---|
| Tree builder | Joins party UUIDs, SSP metadata, components, and inventory items into the federation, organization, boundary, and system tree |
| Ledger | Append-only, hash-chained events for results, attestations, decisions, and expiries |
| Projection engine | Computes the state of any control, cell, or node at any date; materializes horizon buckets |
| Attest and decide | M&O lifecycle, evidence handling, signing, AO risk acceptance, OSCAL and `saf attest` emission |
| Federation | Pulls signed bundles from peer SPARC instances; builds the reverse-inheritance index for blast radius |
| API | chi router implementing [API v0](04-api.md) |

## Repository layout

The Go module is rooted at the **repository**, not at `horizon/`: `go.mod` landed at the top
level with the first package (#38), and the `horizon/` directory is an empty placeholder left
from the original sketch.

```text
  cmd/horizon/          serve | ingest | rebuild | verify
  cmd/genfixtures/      writes fixtures/ — the deterministic federation
  internal/canonical/   field normalisation for derived identifiers
  internal/keys/        the UUIDv5 key grammar, Go reference implementation
  internal/fixtures/    the fixture generator
  internal/sparc/       mTLS client, ETag cache, doc fetch
  internal/oscal/       go-oscal adapters, document round-tripping
  internal/tree/        federation, org, boundary, system builder
  internal/authz/       responsible-parties to node-scoped roles, OIDC
  internal/ledger/      append-only, hash-chained events
  internal/project/     StateAt, rollups, horizon buckets, ranking
  internal/attest/      lifecycle, evidence, signing, saf attest emit
  internal/decide/      risk acceptance, conditions, POA&M emit
  internal/federate/    peer bundles, reverse-inheritance index
  internal/api/         chi router, OpenAPI v0 handlers
  web/                  TypeScript + Vite SPA, embedded via go:embed
  fixtures/             deterministic OSCAL federation, generated (#36)
  deploy/helm/  deploy/terraform/
```

## Dependencies

| Need | Choice | Why |
|---|---|---|
| OSCAL types | `github.com/defenseunicorns/go-oscal`, pinned | **Decided in P0 (#26).** v0.7.1 ships `oscal-1-2-1` and `oscal-1-2-2`; all seven models Horizon reads are present. Five real documents round-trip losslessly apart from timestamp normalisation, and none contained a field the types do not model. The "generate from the NIST schemas" fallback is the same generator self-hosted — `go-oscal` *is* a schema-to-types generator — so it buys no fidelity. **Parse for reading, never to reproduce a signed document:** hash and verify the received bytes |
| HDF types | Generated from the HDF v3 JSON schema | Only needed to read raw HDF from back-matter for provenance views |
| Storage | `modernc.org/sqlite`, Postgres optional | Pure Go, so `CGO_ENABLED=0` and a static binary |
| HTTP | `github.com/go-chi/chi/v5` | Standard `net/http` handlers, small surface |
| Identity | `github.com/coreos/go-oidc/v3` | Works with agency IdPs; groups map to node roles |
| UUIDs | `github.com/google/uuid` (`NewSHA1`), pinned | UUIDv5 over natural keys for idempotent reruns. **In P0 (#36)** it is a direct dependency of `internal/keys` |
| UI | TypeScript, Vite, CSS grid heatmap | No chart library needed for the HUD; add ECharts later for trends only |
