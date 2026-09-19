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
