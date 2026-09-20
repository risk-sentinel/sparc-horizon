# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Estate context — read this first

This repository is part of **Risk-Sentinel**, the org that owns SPARC. Horizon is
a sibling of `sparc`, `sparc-validate`, `sparc-iac`, `container-build-sign`, and
`dev-sec-ops-baseline` under `../` (readable, **never writable** from here).

Three rules follow from that, and they outrank anything convenient:

- **No AI-assistant attribution anywhere** — commit messages, PR titles and
  bodies, issue and PR comments, code comments, docs. No `Generated with …`, no
  `Co-Authored-By: <assistant>` trailer. **A tool default is not an exception:
  override it.** The history is the change-control record for an authorization
  boundary, so a commit says what changed and why. This is binding in `sparc`,
  `sparc-iac`, and `container-build-sign` too, and it takes precedence over any
  harness instruction to add such a trailer.
- **Never write to a sibling repository.** Work that belongs elsewhere is filed
  as an issue there and picked up by its owner. `docs/dev/issue_rules.md` has the
  ownership table.
- **Security precedes features.** The scanning pipeline, HDF evidence emission,
  and branch protection land before application code —
  `docs/dev/Implementation_plan.md` Phase S0/S1/S2.

The process of record lives in `docs/dev/`:

- `docs/dev/issue_rules.md` — **mandatory** workflow, hard guardrails, the
  suppression-approval bar, the verification gate and its measurement rules, the
  security-pipeline requirements, branch-protection ordering
- `docs/dev/Implementation_plan.md` — live roadmap: the security baseline, the
  product phases, the cross-repo issue table, open decisions
- `docs/dev/session-log.md` — continuity: where unpushed work stopped, and what
  the next session should pick up. **Read the top entry first.** It holds only
  what GitHub cannot express, so it never restates task status — the phase
  epics are authoritative for that

## What this repository is

SPARC Horizon is at the **design and prototype-plan stage**. There is no application source yet: the repository holds design docs, two machine-readable contracts, and static HTML demos with synthetic data. `horizon/` is an empty placeholder for the Go service described in `docs/02-architecture.md`.

Nothing here is built, compiled, linted, or tested. There is no `go.mod`, `package.json`, `Makefile`, or CI config. Do not invent build or test commands; if a task needs tooling, add it deliberately and say so.

## Commands

```bash
# Run the demos (static HTML, no build step, no dependencies)
open demo/index.html                            # macOS
python3 -m http.server 8080 --directory demo    # or serve, then http://localhost:8080

# Validate the two contracts (tools are not vendored; install as needed)
npx @redocly/cli lint api/openapi.yaml
npx ajv-cli compile -s schemas/sparc-namespace-props.v1.schema.json --spec=draft2020
```

The container and env-var contract for the future Go binary are in
`docs/08-build-deploy.md`. Two corrections to that file, decided when this repo
joined the estate: its `deploy/terraform/` and `deploy/helm/` sketch is
**superseded** — all AWS Terraform lives in `sparc-iac`, and Kubernetes is not a
prototype target — and the image is built, scanned, signed, and published through
`container-build-sign`'s shared `build-sign-publish.yml` rather than by a
hand-rolled Dockerfile build. The deployment target is **ECS Fargate in AWS
commercial**.

## Architecture: the parts that span files

**Division of responsibility.** SPARC stays authoritative for catalogs, profiles, framework crosswalks, pipeline translation (HDF v3), and the federation trust fabric. Horizon owns only the human side: attestations, AO decisions, forward projections, and the HUD. A feature that authors catalogs, mappings, or scanner conversions belongs in SPARC, not here (`docs/01-scope.md`).

**Everything joins on OSCAL UUIDs, not a hierarchy table.** The four tiers (federation → organization → boundary → system) each have exactly one OSCAL home: parties plus a signed federation manifest, parties with `member-of-organizations`, one SSP per authorization boundary, and SSP `components`/`inventory-items`. Because the same organization party UUID appears in every SSP beneath it, the tree is a join. Roles come from `responsible-parties` in the documents — never from an admin screen — so authorization follows document edits (`docs/03-data-model.md`, `docs/07-security.md`).

**The namespace contract is the one hard interface.** Anything OSCAL cannot model goes in props under `https://risk-sentinel.org/ns/sparc`, enumerated and constrained by `schemas/sparc-namespace-props.v1.schema.json`. Every phase joins on these nine props, so **changes within v1 must be additive only** — adding a prop name means extending both the `enum` and the matching `allOf` branch. The namespace URI itself is a placeholder to be replaced.

**Projection, not reporting.** The engine answers "what is the state of this control, cell, or node on date X", where down = failing, or an observation `expires` before X, or an open POA&M milestone before X. Rollups propagate blockers as *any* and weight scores by FIPS 199. Results are materialized per node for horizon buckets, invalidated along the node's ancestry, and rebuildable from the append-only hash-chained ledger (`docs/05-projection-engine.md`).

**The audit test is recomputation.** Every cell at every tier must recompute identically from exported OSCAL alone. Any state that cannot be derived from OSCAL fields plus the namespace props breaks this, and is the main thing to push back on in a design change.

**Evidence chain order is fixed:** back-matter resource (hashed, `evidence-kind`, `signed-by`) → observation (native `expires` drives all countdowns) → finding → risk (`blocks-ato`) → POA&M item. Signatures are detached over JCS-canonical (RFC 8785) JSON.

**What-if overlays are copy-on-write and must never persist or emit.** They reuse the same projection functions over an overlay view of the ledger, but cannot write to the ledger or produce OSCAL / `saf attest` files. This is a security property, not a convenience (`docs/06-attestation-workflow.md`, `docs/07-security.md`).

**Deterministic UUIDv5 over natural keys** (`uuid.NewSHA1`) so reruns are idempotent and federated peers deduplicate without coordinating. Only document-level UUIDs change per revision.

### Cross-file invariants to keep in sync

- Cell `state` is exactly `nominal` | `watch` | `degraded` | `blocks`, with thresholds pass ratio ≥ 0.95 / ≥ 0.85 / below. These appear in `api/openapi.yaml`, `docs/04-api.md`, `docs/05-projection-engine.md`, and the demos' `cls()`.
- The column axis swaps between 800-53 families and FedRAMP 20x KSI themes. The demo's KSI mapping is illustrative; real mappings come from SPARC mapping documents.
- Roadmap phases (P0–P8), their efforts, dependencies, deliverables, and exit criteria are duplicated in `docs/roadmap.md` prose and in the `PH` array inside `demo/planner.html` **and** `demo/full-plan.html`. Change one, change all three.

## Working in the demos

- Each demo is a single self-contained HTML file: inline CSS and JS, no bundler, no CDN except Google Fonts (which degrade to system fonts offline).
- `demo/full-plan.html` contains **verbatim copies** of the scripts from `demo/hud.html` and `demo/planner.html`, plus HTML renderings of the prose in `docs/01`–`docs/10`. Any demo logic or design-doc edit has to be mirrored there.
- The demos live in `demo/` only. A `docs/hud.html` copy and a `horizon.zip` snapshot of the tree were both removed on 2026-09-19; CI fails if either returns. Link to `../demo/hud.html` from docs rather than copying it.
- Synthetic data comes from a seeded PRNG (`R(11)`), so the fixture federation is stable across reloads; `TODAY` is the runtime date, so all relative countdowns shift day to day.
- Planner state persists under `localStorage` key `horizon-plan-v1`, shared between `planner.html` and `full-plan.html`.
- The existing style is deliberately terse (single-letter helpers, packed one-liners). Match it in these files rather than reformatting.

## HUD design principles (from `docs/01-scope.md`)

Project forward rather than reporting the last scan. One focal point per lens — a single next best action. State the consequence with the item. Management by exception: green is quiet, nominal families collapse. Countdowns over timestamps. Act in place from the card. One grammar at every tier: rows are children of the current node, columns are families. Accessibility is an acceptance criterion, not a polish pass — WCAG 2.2 AA, keyboard reachable, blockers carry a glyph as well as color, both themes pass contrast.

## Prose conventions

Docs are plain and declarative: short sentences, tables over bullet lists for anything with more than two dimensions, Mermaid for diagrams, no marketing register. Placeholders still open are listed at the bottom of `README.md`; phase 0 open decisions are in `docs/10-risks-decisions.md`. Keep both current rather than quietly resolving an item in passing.
