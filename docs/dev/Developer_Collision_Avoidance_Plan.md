# SPARC-Horizon Developer Collision Avoidance Plan

Companion to [`Implementation_plan.md`](Implementation_plan.md). Maps work to exact
files and domains, defines branching rules, and documents file-touch hot spots so
work can parallelize without collisions.

**Last updated:** 2026-09-19 (**repository preparation.** No application code
exists, so the collision surface today is entirely docs, contracts, demos, and the
`.github/` tree being stood up in Phase S0. **The hot files are the three
duplication axes described below** — they are the only places in this repository
where a one-line change has to be made in more than one file to be correct, and
they will cause every avoidable conflict until they are removed.)

---

## 1. Domain ownership map

| Domain | Paths | Owner phase | Notes |
|---|---|---|---|
| Design of record | `docs/01-scope.md` … `docs/10-risks-decisions.md` | all | Changing a contract here is a change to the product, not documentation housekeeping |
| Roadmap | `docs/roadmap.md` | all | **Duplicated** into two demo files — see hot files |
| Process | `docs/dev/*.md` | all | `issue_rules.md` is binding; this file and `Implementation_plan.md` update on every issue |
| API contract | `api/openapi.yaml` | P0, P2–P5 | Frozen at end of P0; changes after that are versioned, not edited in place |
| Namespace contract | `schemas/sparc-namespace-props.v1.schema.json` | P0 | **Additive only within v1.** A new prop touches the `enum` and a matching `allOf` branch |
| Demos | `demo/*.html`, `docs/hud.html` | S0, P3 | Synthetic data, no build step. **Duplicated** — see hot files |
| Pipeline | `.github/workflows/`, `.github/actions/`, `.security/`, `container-baseline.yml` | S0, S1 | One workflow PR at a time (rule below) |
| Compliance artefacts | `docs/compliance/` | S0-15, then every security-touching issue | CDEFs, the NIST mapping, inline control comments |
| Go service | `cmd/horizon/`, `internal/*` | P1–P7 | Does not exist yet; ownership splits by package, per `docs/02-architecture.md` |
| Web UI | `web/` | P3, P7 | Does not exist yet |

### Package-level ownership, once code lands

The layout in `docs/02-architecture.md` is deliberately one package per concern,
which makes most product phases collision-free against each other:

| Package | Phase | Collides with |
|---|---|---|
| `internal/sparc`, `internal/oscal`, `internal/tree`, `internal/authz` | P1 | Each other only |
| `internal/ledger`, `internal/project` | P2 | P7 reads `project`; do not refactor its signatures while P7 is open |
| `internal/attest` | P4 | `internal/ledger` writers |
| `internal/decide` | P5 | `internal/attest` lifecycle states |
| `internal/federate` | P6 | `internal/project` rollups |
| `internal/api` | P1–P6 | **Everything.** The router is the one file every phase adds to |
| `web/` | P3, P7 | P7 adds the simulation banner to P3's shell |

`internal/api` is the predictable conflict. Keep route registration one line per
route in a stable alphabetical block so two phases adding routes conflict on
adjacent lines rather than the same line.

---

## 2. Hot files

These are ranked by how likely a change is to be *silently* incomplete rather
than by how often they are touched.

### 2.1 `demo/full-plan.html` — verbatim copies of two scripts and ten documents

`demo/full-plan.html` contains, line for line:

- the entire script body of `demo/hud.html` (verified identical apart from the
  IIFE wrapper and one blank line)
- the entire script body of `demo/planner.html` (identical apart from four
  trailing event-handler lines)
- HTML renderings of the prose in `docs/01` through `docs/10`

So a change to HUD logic, planner logic, or any design doc is **three edits**, and
the file is 66 KB, so a reviewer will not notice the omission. Anything in flight
on any of those sources conflicts here.

**Rule:** a PR that touches `demo/hud.html`, `demo/planner.html`, or `docs/0*.md`
states in its Test plan whether `full-plan.html` needed the same change, and why
if not.

### 2.2 `docs/hud.html` — byte-identical copy of `demo/hud.html`

Two copies of the same 26 KB file, no generator, nothing asserting they match.
Scheduled for removal in **S0-14**; until then, both change together.

### 2.3 Roadmap phase data — three copies

`docs/roadmap.md` prose, the `PH` array in `demo/planner.html`, and the same array
in `demo/full-plan.html` all carry each phase's effort, dependencies, tasks,
deliverables, exit criteria and risks. A phase edit is three edits.

### 2.4 `docs/dev/Implementation_plan.md` and this file

Every issue updates both, per `issue_rules.md` step 8. Two concurrent branches
will conflict here, always, in the status snapshot and the **Last updated** line.
This is expected and cheap — resolve by keeping both entries, newest first. It is
not a reason to skip the update.

### 2.5 `.github/workflows/` during S0 and S1

Workflows are added one at a time by design (`pipeline-hardening`: add a scanner,
confirm what it measured, then add the next). Two workflow PRs open at once
defeats the point, because neither has been observed reporting in isolation.

**Rule: one workflow PR at a time.** The exception is a workflow being added
alongside the fixture or config it needs (`secret-scan.yml` with
`tests/trufflehog-fixture/` and `.trufflehog-exclude-paths`), which is one change.

---

## 3. Branching strategy

Per [`issue_rules.md`](issue_rules.md):

- `feature/<issue>_slug` or `bug/<issue>_slug`, issue number in the branch name
- One PR per issue; multiple commits per PR is fine
- Never push to `main`; only the owner merges
- **If the current branch is not `main`, ask before cutting another.** A second
  branch from `main` gets a stale copy of every shared file in this document's
  section 2, and an edit made against a stale copy has to be redone rather than
  cherry-picked

### Supporting changes ride the open bundle

New work arriving mid-bundle goes **on the current branch** by default. The shared
files above are exactly why: `Implementation_plan.md`, this file, and `CLAUDE.md`
are touched by nearly every issue, so a parallel branch guarantees a conflict in
files that carry process rather than code.

---

## 4. Parallel work capacity

| Window | Safe in parallel | Why |
|---|---|---|
| **Now (S0)** | 1–2 streams | Docs/contracts vs `.github/` tree. Both touch `Implementation_plan.md`, so expect a trivial conflict there |
| **S1 + P1/P3** | 2–3 | S1 workflows, P1 Go packages, P3 `web/` are disjoint trees. S1 still serialises within itself |
| **P2** | 1–2 | P2 owns `internal/ledger` + `internal/project`; P7 must not start until its signatures settle |
| **P4/P5** | 2 | `internal/attest` and `internal/decide` are separate packages sharing a lifecycle contract; agree the states before both start |
| **P8** | 3 | Mostly packaging and pilot rehearsal, per `docs/roadmap.md` |

The roadmap's planner already models this (up to `par` people per phase, plus 10%
coordination overhead per extra person). Re-plan with `demo/planner.html` rather
than re-deriving it here.

---

## 5. Cross-repo open work

Horizon writes to no sibling repository. Filed work, owned elsewhere:

| Issue | Repo | Blocks here |
|---|---|---|
| [sparc-iac#715](https://github.com/risk-sentinel/sparc-iac/issues/715) | `sparc-iac` | S0-11 emit role; S2 deployment. Hub for the boundary pivot |
| [sparc#1153](https://github.com/risk-sentinel/sparc/issues/1153) | `sparc` | Nothing here — coordination only |
| [sparc-validate#400](https://github.com/risk-sentinel/sparc-validate/issues/400) | `sparc-validate` | Nothing here — coordination only |
| [container-build-sign#325](https://github.com/risk-sentinel/container-build-sign/issues/325) | `container-build-sign` | Nothing here — coordination only |
| [#1](https://github.com/risk-sentinel/sparc-horizon/issues/1) | this repo | S0 exit |

Still to file (X-1, X-2, X-4, X-5, X-6 in `Implementation_plan.md`): the org
inventory declaration, the ECR repo and consumer registration, the namespace
validate rules and mapping documents, deployed-resource profile execution, and the
registered namespace URI.

---

## 6. Collision risks, ranked

| Risk | Likelihood | Mitigation |
|---|---|---|
| A demo or doc change lands in one of two or three copies | **High** — it is the default outcome without a check | S0-14 removes one copy; the Test-plan question in 2.1 catches the rest; a CI diff check is the real fix |
| Two workflow PRs open at once, neither observed reporting alone | Medium | One workflow PR at a time |
| A required check added before it reports, blocking every PR | Medium, and **unrecoverable without owner bypass** | S0/S1 split; read the reported name from the forge, never the workflow file |
| `internal/api` route registration conflicts | Medium, once code lands | Stable alphabetical one-line-per-route block |
| `Implementation_plan.md` / this file conflict | Certain | Keep both entries, newest first |
| Namespace schema edited non-additively | Low, high cost | v1 is additive only; a breaking change is v2 and a `sparc` issue first |

---

## Summary

Today the only real collision surface is **duplication that nothing checks**:
`full-plan.html` against two demos and ten docs, `docs/hud.html` against
`demo/hud.html`, and roadmap phase data across three files. None of it is load-
bearing for the product, and all of it is load-bearing for whether a change is
correct. S0-14 removes one axis; the other two want a CI check that diffs the
copies, which is worth filing once the `.github/` tree exists.
