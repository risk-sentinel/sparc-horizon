# SPARC Horizon

A heads-up display for authorization risk. Horizon shows authorizing officials (AO), system owners (SO), and information security officers (ISO) what is ahead of each boundary, and lets them act on the management and operational (M&O) controls that automation cannot cover.

Horizon sits on SPARC's Delivery layer. SPARC stays authoritative for pipeline translation, framework crosswalks, and the federation trust fabric. Horizon owns the human side: attestations, AO decisions, forward projections, and the HUD.

> **Status:** design and prototype plan. "SPARC Horizon" is a working name.

## Quick demo

The demos are static HTML with synthetic data. Nothing to build or install.

```bash
# open directly
open demo/index.html            # macOS
xdg-open demo/index.html        # Linux

# or serve locally
python3 -m http.server 8080 --directory demo
# then browse to http://localhost:8080
```

| Demo | What it shows |
|---|---|
| [`demo/hud.html`](demo/hud.html) | Tiered heatmap (federation, organization, boundary, system), look-ahead slider, drill-in to the OSCAL evidence chain, what-if attestations, and an 800-53 or KSI column axis |
| [`demo/planner.html`](demo/planner.html) | Adjustable phase schedule: team size, start date, buffer, scope, and per-phase effort |
| [`demo/full-plan.html`](demo/full-plan.html) | Both demos plus the full design on a single page |

Fonts load from Google Fonts when online and fall back to system fonts offline. Planner settings persist in the browser's `localStorage`.

## Documentation

| Doc | Covers |
|---|---|
| [01 Scope](docs/01-scope.md) | The claim the prototype proves, what is in and out |
| [02 Architecture](docs/02-architecture.md) | Layers, components, repository layout, dependencies |
| [03 Data model](docs/03-data-model.md) | Tier-to-OSCAL mapping, namespace contract, evidence chain, UUIDs, inheritance |
| [04 API](docs/04-api.md) | API v0 endpoints and response shapes ([`api/openapi.yaml`](api/openapi.yaml)) |
| [05 Projection engine](docs/05-projection-engine.md) | State at any date, rollups, cell rules, next-best-action ranking |
| [06 Attestation workflow](docs/06-attestation-workflow.md) | M&O lifecycle, `saf attest` round trip, AO decisions |
| [07 Security](docs/07-security.md) | Identity, node-scoped authorization, integrity, federation |
| [08 Build and deploy](docs/08-build-deploy.md) | Container, configuration, Terraform |
| [09 Acceptance](docs/09-acceptance.md) | Acceptance criteria and pilot demo script |
| [10 Risks and decisions](docs/10-risks-decisions.md) | Risks, mitigations, and phase 0 decisions |
| [Roadmap](docs/roadmap.md) | Phase plan with tasks, exit criteria, and a default schedule |
| [Compliance](docs/compliance/README.md) | Horizon's own control story: the 800-53 mapping, its OSCAL component definitions, and the threat model attestation |

Machine-readable contracts:

- [`schemas/sparc-namespace-props.v1.schema.json`](schemas/sparc-namespace-props.v1.schema.json): the namespace props Horizon depends on
- [`api/openapi.yaml`](api/openapi.yaml): API v0 skeleton
- [`docs/compliance/oscal/cdefs/`](docs/compliance/oscal/cdefs/): OSCAL 1.2.x component definitions for Horizon itself

## Placeholders to replace

- The namespace URI `https://risk-sentinel.org/ns/sparc` is illustrative.
- The KSI theme mapping in the demo is illustrative; real mappings come from SPARC.
- `go-oscal` coverage of OSCAL 1.2.x is a phase 0 decision, not an assumption.
