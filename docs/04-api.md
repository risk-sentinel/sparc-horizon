# 04 API v0

Frozen as OpenAPI at the end of phase 0 so interface work can start on mocks. Skeleton: [`api/openapi.yaml`](../api/openapi.yaml).

| Method and path | Purpose |
|---|---|
| `GET /v1/tree` | Nodes visible to the caller, with role bindings |
| `GET /v1/nodes/{id}/heat?horizon=23d&axis=800-53` | Rows, ordered columns, collapsed families, cell states |
| `GET /v1/nodes/{id}/cell?col=CP&horizon=23d` | Controls down in a cell, ranked, with reasons |
| `GET /v1/controls/{uuid}/chain` | Evidence, observation, finding, risk, POA&M |
| `GET /v1/nodes/{id}/next-action` | Top-ranked action for this lens |
| `POST /v1/attestations` | Submit an M&O attestation |
| `POST /v1/attestations/{id}/sign` | Sign a reviewed attestation |
| `POST /v1/decisions` | AO risk acceptance with conditions |
| `POST /v1/whatif` | Create a copy-on-write overlay; returns an overlay ID for heat calls |
| `GET /v1/nodes/{id}/blast-radius` | Leveraging boundaries affected by a provider cell |
| `GET /v1/export/oscal/{boundary}` | AR and POA&M for any GRC to render |

All node-scoped endpoints check the caller's role on the node, not on the endpoint. See [07 Security](07-security.md).

**Frozen at the end of P0.** [`api/openapi.yaml`](../api/openapi.yaml) is the contract: request
and response schemas for all eleven paths, a shared error shape, and the refusal rule below.
Changes after this are versioned rather than edited in place, because P1's client and P3's HUD
are both built against it.

## A refusal is a 404, never a 403

A node the caller holds no role on returns **404**, identical to a node that does not exist.

Horizon's tree spans organizations that are not meant to see each other. A 403 confirms that a
boundary exists, that it is called something in particular, and by inference who owns it — a
disclosure a federated HUD should not make. The cost is accepted and written down rather than
left to be rediscovered: a mistyped node id is indistinguishable from one the caller may not see.

The same rule applies one level down. A chain is reached by implemented-requirement UUID, and a
requirement whose component the caller cannot see is refused the same way a node is.

## The mock

`go run ./cmd/mockserver` serves the contract over the fixture federation, from goldens in
[`fixtures/api/`](../fixtures/api/README.md) that are derived by parsing the OSCAL the generator
emitted. An `X-Horizon-Persona` header selects the caller; each persona sees a **different tree**
rather than the same tree with parts greyed out, which is what makes the 404 rule coherent.

It does not project: cell states are computed once, at generation time, for one horizon, and the
`horizon` parameter is accepted and ignored. It answers the write endpoints with 501 rather than
accepting a write and forgetting it. CI asserts that every golden validates against the schema
its path declares, and that the contract declares nothing the mock cannot answer.

## Heat response

```json
{
  "node": "enterprise-it",
  "horizon": "2026-10-11",
  "axis": "800-53",
  "columns": ["CP", "RA", "SI", "AC", "CM"],
  "collapsed": ["AU", "IA", "IR", "PS", "SC"],
  "rows": [
    {"id": "portal", "name": "Portal", "cells": [
      {"col": "CP", "state": "blocks", "blockers": 2, "worsening": true, "inherited": false},
      {"col": "RA", "state": "watch", "blockers": 0, "worsening": true, "inherited": false}
    ]}
  ]
}
```

Cell `state` is one of `nominal`, `watch`, `degraded`, or `blocks`. Keep cell payloads this small; details come from the cell and chain endpoints.

A row carries a cell only for the columns that survived collapsing, so `cells` is a subset of
`columns` — the example above has five columns and two cells. A family a row has nothing in is
omitted rather than drawn as passing: they are not the same claim.

**The `800-53` axis carries NIST families only.** Identifiers from another authority — AWS
Security Hub arrives through inherited component definitions — are off this axis until SPARC's
crosswalk maps them (`sparc#1103`). Placing `ACM` beside `AC` would claim a mapping this
repository does not own.
