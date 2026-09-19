# SPARC-Horizon Implementation Plan

Structured, prioritized roadmap for the sparc-horizon repository. Horizon is the
Delivery-layer HUD for authorization risk: it owns attestations, AO decisions,
forward projections, and the role lenses, while SPARC stays authoritative for
catalogs, crosswalks, pipeline translation, and the federation trust fabric.

**Last updated:** 2026-09-19 (**repository joins the estate.** Horizon arrived as
a wire-frame: ten design docs, an OpenAPI v0 skeleton, a namespace JSON Schema,
and three static HTML demos with synthetic data — no application code, no
pipeline, no protection, and no declaration in the org inventory. This plan
inserts a **security and supply-chain baseline (Phase S0/S1/S2) ahead of all
product work**, supersedes the in-repo Terraform sketch in
[`docs/08-build-deploy.md`](../08-build-deploy.md) in favour of `sparc-iac`, and
fixes the deployment target as **ECS Fargate in AWS commercial**. The product
phases P0–P8 carry over from [`docs/roadmap.md`](../roadmap.md) unchanged in
content, re-sequenced behind S0.)

---

## Guiding Principles

- **Security before features.** The pipeline, the evidence path, and branch
  protection land before application code. A prototype is a prototype of how we
  build, not only of what we built.
- **Green has to mean something.** Every check must be able to fail. A scanner
  that assessed nothing and a clean repository must not produce the same
  artifact — assert the count, not the exit code.
- **HDF-native.** Every scanner output converts to Heimdall Data Format and
  reaches the evidence store, including on a clean run. An absent artifact and a
  clean one must not look the same.
- **Horizon dogfoods.** It runs through the same reusable SAF pipeline it
  projects for everyone else and produces its own HDF and OSCAL package. Its own
  posture has to survive the read it performs on others.
- **Recompute from OSCAL is the audit test.** Every cell at every tier
  recomputes identically from exported OSCAL alone. State that cannot be derived
  from OSCAL fields plus the namespace props does not belong in the engine.
- **The namespace contract is additive-only within v1.** Every phase joins on
  those nine props.
- **Cross-repo work is filed, not done.** Horizon reads the estate and writes
  nothing in it. See [Cross-repository issues](#cross-repository-issues).
- **One binary, static, distroless.** `CGO_ENABLED=0`, pure-Go SQLite, embedded
  TypeScript UI via `go:embed`.
- **AWS commercial, ECS Fargate.** Matching the existing SPARC deployment and
  `rs-aws-ecs-fargate-baseline`. Kubernetes is out of scope for the prototype.

---

## Issue Process

See **[`issue_rules.md`](issue_rules.md)** for the complete mandatory workflow,
hard guardrails, the suppression-approval bar, the verification gate and its
measurement rules, the security pipeline requirements, and the branch-protection
ordering rule.

---

## Status snapshot

> **Updated 2026-09-19.** Everything below is a starting value, not a regression.
> The repository is one commit old.

| Bucket | Current state |
|---|---|
| Application code | **None.** `horizon/` is an empty placeholder for the layout in [`docs/02-architecture.md`](../02-architecture.md) |
| Design docs | **11** (`docs/01`–`docs/10` + `roadmap.md`) — the design of record |
| Machine-readable contracts | **2** — `api/openapi.yaml` (v0 skeleton), `schemas/sparc-namespace-props.v1.schema.json` (v1, 9 props) |
| Demos | **4** static HTML files, synthetic data, seeded PRNG, no build step |
| CI workflows | **4** — `secret-scan.yml` (gate + fixture canary), `secret-scan-hdf-emit.yml` (emitter, fails closed), `pr-checklist.yml`, `contracts.yml` (OpenAPI, namespace schema, actionlint, duplication drift) |
| Branch protection | **None yet** — correct at this point: the four workflows have not run on `main`, so no context has been observed reporting. See S0-12 |
| Secret scanning | **Gate + canary landed.** TruffleHog verified-only, `tests/trufflehog-fixture/` planted and asserted, exclude file scoped to the fixture alone |
| SAST / code scanning | **None** — no CodeQL, no SonarCloud project, no `golangci-lint` config |
| Dependency / SBOM / SCA | **None** — no `go.mod` yet, no Dependabot config, no `.security/sca-allowlist.yaml` |
| Container | **None** — the Dockerfile in [`docs/08-build-deploy.md`](../08-build-deploy.md) is a sketch, unpinned, never built |
| HDF evidence emitted | **0 artifacts.** Emitter written and fails closed on an unset boundary or absent role; blocked on the `risk-sentinel/*/sparc-horizon/*` role (`sparc-iac#715`) |
| Org inventory (`dev-sec-ops-baseline`) | **Not declared.** `devsecops-inventory-reconciliation` does not see this repo |
| `container-build-sign` consumer list | **Not listed.** No ECR repo, no signed image, no pin-bump notifications |
| AWS deployment | **None.** No `sparc-iac` module, no emit role, no task definition |
| NIST control coverage (application layer) | **0 documented.** No `docs/compliance/` tree yet |
| Highest-priority next work | **Phase S0** — the pre-code security baseline. It fits in the window before the roadmap's 2026-10-05 P0 start, so it costs nothing on the product critical path if it starts now |

---

### Landed 2026-09-19

- **Estate onboarding docs.** `issue_rules.md` (ported and de-Railsed — the copy that
  arrived carried `bundle exec rspec`, ActiveRecord migration rules, the wiki
  publishing arc, and SPARC's auth-mode matrix), `Implementation_plan.md`,
  `Developer_Collision_Avoidance_Plan.md`, `CONTRIBUTING.md`, `CLAUDE.md`.
- **Evidence boundary decided: `risk-sentinel`**, and filed as a coordinated
  five-repo pivot — `sparc-iac#715` (hub: IAM, bucket policy, org variable),
  `sparc#1153`, `sparc-validate#400` (+ its 20-repo fleet), `container-build-sign#325`,
  `sparc-horizon#1`. Measured while scoping it: the org variable
  `EVIDENCE_BOUNDARY = sparc` is the single source every producer reads, so the flip
  itself needs no PR in 25 repositories — but **24 hardcoded `|| 'sparc'` fallbacks**
  sit behind it, and `sparc-validate`'s fleet carries the boundary as a *committed*
  value in `.gitlab-variables.yml`, which no GitHub org variable reaches. That second
  one is how the two forges could file evidence under different boundaries with both
  pipelines green.
- **Secret scanning, gate and canary** (S0-1, S0-3). The canary is the load-bearing
  half: without it a regressed scanner and a clean repository are the same artifact.
- **HDF emitter** (S0-2), failing closed on an unset boundary. The reusable's
  `boundary` input is `required` and deliberately undefaulted for this reason, but an
  empty string satisfies `required: true`, so the preflight check lives here.
- **PR machinery** (S0-4, S0-5, S0-6): CODEOWNERS over the security-relevant paths,
  the five-section template, the checklist gate.
- **`contracts.yml`** (S0-7, S0-8) — and it asserts counts rather than exit codes:
  the OpenAPI spec must declare a non-zero path count (11 today), actionlint must find
  workflows to lint, and the namespace schema is checked in **both** directions —
  9 documented prop examples must validate, 8 malformed ones must be rejected.
  Verified locally: 17 passed, 0 failed.
- **Duplication drift guard** — the three copies this repository carries
  (`docs/hud.html` vs `demo/hud.html`, `demo/full-plan.html`'s verbatim demo scripts,
  roadmap phase data in three files) now fail CI when they diverge. All three pass
  against the current tree.
- **`horizon.zip` dropped** (S0-13) and `*.zip` ignored.

**Not started, and deliberately so:** S0-12 branch protection, because no check has
been observed reporting on `main` yet — requiring one before that is the trap that
blocks every PR permanently. S0-9 SonarCloud, S0-14 copy removal, S0-15 compliance
skeleton, S0-17 threat model remain.

---

## Sequencing: why the security baseline splits in two

The obvious plan — "stand up every scanner, then turn on branch protection, then
build" — cannot be executed in that order on a repository with no application
code, and attempting it bricks the repository.

CodeQL language detection and SonarCloud analysis both key off the **default
branch**. With no Go or TypeScript on `main`, they detect nothing, so they report
nothing. A required check that never reports blocks every pull request forever,
and the block presents as "expected, waiting for status" — including the very PR
that would land the code and fix it.

So the baseline splits:

- **Phase S0 — pre-code.** Everything that can be true of a repository with no
  application code: secret scanning and its detection fixture, the PR and
  CODEOWNERS machinery, the contract linters, the evidence path, the org
  declaration, and protection covering **only the checks that demonstrably
  report**.
- **Phase S1 — with the first code.** The scanners that need a language on
  `main`: SAST, code scanning, SonarCloud, SBOM, SCA, `govulncheck`, the signed
  container. Each is added one at a time, observed reporting, and only then
  required. S1 interleaves with product phase P1, which produces the first Go
  package.
- **Phase S2 — deployment baseline.** Mostly `sparc-iac`'s work, filed as issues
  and tracked here.

Nothing in P2 onward starts before S1 is complete.

---

## Phase S0 — Repository and security baseline (pre-code)

Make the repository a first-class estate member before any code lands.

- **Base effort:** 2 engineer-weeks, up to 2 people
- **Depends on:** nothing
- **Target window:** 2026-09-22 to 2026-10-03 (ahead of the P0 start)

| ID | Task | Issue | Done |
|---|---|---|---|
| S0-1 | TruffleHog verified-only gate (`secret-scan.yml`) **plus** the `fixture-detection` job that scans `tests/trufflehog-fixture/` and asserts the planted synthetic key is found. Ported from `sparc-validate`/`container-build-sign` so the estate runs one scanner with one configuration | | 2026-09-19 |
| S0-2 | `secret-scan-hdf-emit.yml` as a caller of `dev-sec-ops-baseline`'s reusable `secret-scan-hdf.yml`. **A clean scan still emits** a one-control HDF recording the execution. Never runs on `pull_request` | | 2026-09-19 |
| S0-3 | `.trufflehog-exclude-paths` scoped to the fixture only | | 2026-09-19 |
| S0-4 | `CODEOWNERS` covering `docs/`, `api/`, `schemas/`, `.github/`, and (once they exist) `internal/authz/`, `internal/attest/`, `internal/decide/` | | 2026-09-19 |
| S0-5 | `.github/PULL_REQUEST_TEMPLATE.md` with the five-section shape, and `pr-checklist.yml` failing on any unchecked `- [ ]` outside `## Test plan` | | 2026-09-19 |
| S0-6 | `CONTRIBUTING.md` documenting the PR convention and pointing at `issue_rules.md` | | 2026-09-19 |
| S0-7 | Contract CI: Redocly lint on `api/openapi.yaml`, ajv compile on the namespace schema, and a check that every `props[]` example in the design docs validates against it | | 2026-09-19 |
| S0-8 | Workflow linting (`actionlint`) in CI — a schema error yields a 0-second run with no jobs and no logs, so it must be caught in the change that introduces it | | 2026-09-19 |
| S0-9 | `sonarqube-hdf-emit.yml` copied in self-contained (a public repo cannot call a private reusable), CONFIGURATION block set to `REPO_SLUG: sparc-horizon`. It verifies the project **resolves** before fetching, so it fails rather than reporting a clean empty result | | |
| S0-10 | `.github/dependabot.yml` for `github-actions` now; `gomod` and `npm` ecosystems added in S1 when the manifests exist | | 2026-09-19 |
| S0-11 | Evidence path proven end to end with the secrets HDF: prefix `risk-sentinel/<date\|latest>/sparc-horizon/secrets/`, provenance stamped, and the **landed object read back and verified** rather than trusting a green upload. No `\|\| 'sparc'` fallback — fail closed on an unset boundary | [#1](https://github.com/risk-sentinel/sparc-horizon/issues/1) | |
| S0-12 | Branch protection: ruleset copied from `sparc-validate`, `strict_required_status_checks_policy: true`, signed commits, reviews required, bypass **pull request only**, enforcement **active** not evaluate. Required contexts limited to the checks observed reporting: secret scan, fixture detection, PR checklist, contract lint, actionlint | | |
| S0-13 | Delete `horizon.zip` from history-going-forward and gitignore it; it is a snapshot of the repo that goes stale on every commit *(already dropped from the working tree 2026-09-19 — confirm the ignore rule)* | | 2026-09-19 |
| S0-14 | De-duplicate `docs/hud.html` (byte-identical to `demo/hud.html`) — one copy, or a generated one with the generator in CI | | |
| S0-15 | `docs/compliance/` skeleton: `README.md`, `nist-sp800-53-rev5-mapping.md`, `oscal/cdefs/`, and the inline-control-comment format. Horizon's own control story starts empty and grows per issue, per `issue_rules.md` step 9 | | |
| S0-16 | `docs/dev/Developer_Collision_Avoidance_Plan.md` — domain ownership and hot files | | 2026-09-19 |
| S0-17 | Threat model and security architecture review recorded as a dated, signed attestation document. This is the one stage `dev-sec-ops-baseline` deliberately does not automate: it produces a document and a conversation, and is evidenced through the attestation path where freshness is asserted | | |

### Exit criteria

- A planted secret fails the gate; the fixture job fails if the scanner regresses.
- A clean secret scan produces a valid HDF in the bucket at the canonical key,
  with provenance, and the object has been read back.
- A PR with an unchecked box outside `## Test plan` is blocked.
- A direct push to `main` is refused for the owner as well.
- `gh api repos/risk-sentinel/sparc-horizon/rules/branches/main` shows the
  ruleset **active**, and a test PR is blocked for the expected reason.
- The repo is declared in `dev-sec-ops-baseline`'s inventory and
  `devsecops-inventory-reconciliation` passes.

### Risks

- Requiring a check that cannot yet report bricks the repository — mitigated by
  the S0/S1 split and by reading reported names from the forge.
- Horizon's emit is blocked until the `risk-sentinel/*/sparc-horizon/*` emit role
  exists (`sparc-iac#715`). S0 can still complete: land the emit workflow in a
  state that **fails closed** on the missing role, which is the correct behaviour
  anyway and is verifiable without the role existing.

---

## Phase S1 — Pipeline completion (with the first application code)

Add the language-dependent scanners one at a time, each observed reporting before
it is required.

- **Base effort:** 2 engineer-weeks, up to 2 people
- **Depends on:** S0, and the first Go package from P1
- **Interleaves with:** P1

| ID | Task | Issue | Done |
|---|---|---|---|
| S1-1 | `go.mod` at the Go version pinned in [`docs/08-build-deploy.md`](../08-build-deploy.md), `.golangci.yml` with a pinned `golangci-lint`, `gosec` enabled | | |
| S1-2 | `ci.yml`: `gofmt -l`, `go vet`, `golangci-lint`, `go test ./... -race` with coverage. Assert the package count and coverage number — a package with no tests exits 0 and prints `no test files` | | |
| S1-3 | CodeQL for `go` and `javascript-typescript`; assert the detected language list is non-empty | | |
| S1-4 | SonarCloud project onboarded (`risk-sentinel_sparc-horizon`); compare analysed lines against the tree — indexed is not analysed | | |
| S1-5 | `govulncheck` on every PR that touches `go.mod`/`go.sum`, not only at release. A transitive advisory gets no Dependabot PR, so the queue being empty is not evidence | | |
| S1-6 | `sbom-and-sca.yml` calling `container-build-sign`'s `sbom-source.yml` + `sca-scan.yml`, SHA-pinned; `.security/sca-allowlist.yaml` starting **empty** as the intended steady state, every future entry carrying an expiry | | |
| S1-7 | `web/` CI: `tsc --strict`, ESLint, unit tests, axe accessibility run that gates | | |
| S1-8 | Dockerfile hardened from the sketch: base images pinned **by digest**, non-root, distroless, `CGO_ENABLED=0`, `-trimpath`. hadolint in CI | | |
| S1-9 | Consume `container-build-sign`'s `build-sign-publish.yml` — Trivy gate against the image that ships, cosign signature, CycloneDX attestation, ECR publish. ECR-only (`publish_to_dockerhub: false`) unless the owner wants a public image | | |
| S1-10 | `container-baseline.yml` for CRITICAL/HIGH dispositions, each with `rationale`, `nist_control`, `reviewed_by`, `next_review_date` | | |
| S1-11 | Playwright smoke suite (Chrome, zero CSP violations) against the built image, run detached and polled — not concurrently with the Go suite | | |
| S1-12 | `required-checks.json` + an aggregating `required-passed.yml` if path-filtered checks start leaving PRs waiting, as they did in `sparc` (#436) | | |
| S1-13 | Extend branch protection with the now-reporting contexts, read from the forge | | |
| S1-14 | `docs/security/SCANNER_FINDINGS_AUDIT.md` and the release-time refresh cadence: every release PR re-runs the scanners, reconciles counts and the suppression inventory, and confirms no suppression's review date is older than 90 days | | |

### Exit criteria

- Every scanner class in `issue_rules.md`'s requirements table reports, and each
  has been shown failing on a known-dirty input.
- The published image verifies by cosign and its attestation SBOM has a non-zero
  component count.
- `dev-sec-ops-baseline`'s `devsecops-coverage-*` controls pass for `sast`,
  `secrets`, `sca`, `sbom`, `container`, and `test_execution`.
- HDF lands for every class, on clean runs too.

---

## Phase S2 — Deployment baseline (AWS commercial)

Mostly filed work. Horizon owns the container and the configuration contract;
`sparc-iac` owns everything that runs it.

- **Base effort:** 1 engineer-week here, plus `sparc-iac`'s own effort
- **Depends on:** S1

| ID | Task | Owner | Issue | Done |
|---|---|---|---|---|
| S2-1 | ECR repository `sparc-horizon`, immutable tags, no `:latest` | `container-build-sign` / `sparc-iac` | | |
| S2-2 | Emit role `SPARC_HORIZON_EMIT_ARN`, scoped to `<boundary>/*/sparc-horizon/*` | `sparc-iac` | | |
| S2-3 | ECS Fargate service, task definition, ALB, secrets wiring for `HORIZON_*`, evidence bucket access | `sparc-iac` | | |
| S2-4 | OIDC application registration with the agency IdP; `HORIZON_OIDC_ISSUER` | owner / `sparc-iac` | | |
| S2-5 | Signing certificate provisioning, and a dev CA behind the same interface for the prototype environment | `sparc-iac` | | |
| S2-6 | `rs-aws-ecs-fargate-baseline` + `rs-aws-secrets-baseline` executed against the deployed service, HDF to the evidence store | `sparc-validate` | | |
| S2-7 | Horizon's own OSCAL package produced by the pipeline it runs through | this repo | | |
| S2-8 | Configuration contract kept current in [`docs/08-build-deploy.md`](../08-build-deploy.md), and the in-repo Terraform/Helm sketch removed | this repo | | |

### Exit criteria

- The image ECS runs is the signed digest the pipeline published.
- Deployed-resource InSpec results reach the evidence store
  (`artifact-inspec-deployed`).
- No Horizon secret exists outside Secrets Manager or the CI secret store.

---

## Product phases

Carried from [`docs/roadmap.md`](../roadmap.md), which remains the source for
effort, dependencies, tasks, deliverables, and exit criteria. **That file's phase
data is duplicated in `demo/planner.html` and `demo/full-plan.html` — change one,
change all three.** The only change here is the dependency on the security
baseline and the deployment target.

| Phase | Name | Effort | Depends on | Notes |
|---|---|---|---|---|
| P0 | Contracts and fixtures | 3 ew | **S0** | Namespace schema v1 already exists; P0 adds the `sparc-validate` rules (filed there), the UUIDv5 key grammar, the fixture federation, and freezes API v0 |
| P1 | SPARC client and tree builder | 4 ew | P0 | First Go code — carries **S1** with it |
| P3 | HUD heatmap interface | 6 ew | P0 | Built against the mock; first TypeScript — also carries S1 |
| P2 | Ledger, rollup, projection engine | 4 ew | P1, **S1** | The recompute-from-OSCAL audit test is a gate from here on |
| P4 | M&O attestation workflow | 4 ew | P2 | Signing lands here; S2-5 must precede the real-PKI leg |
| P7 | What-if mode *(optional, in default scope)* | 2 ew | P2, P3 | A test must prove an overlay writes nothing to the ledger |
| P5 | AO decisions and POA&M *(optional, in default scope)* | 3 ew | P4 | |
| P6 | Federation sync and blast radius *(optional, not in default scope)* | 4 ew | P2 | |
| P8 | Hardening, packaging, and pilot | 3 ew | all included, **S2** | Much of the original P8 packaging work moves into S1/S2; what remains is the pilot and the audit export |

**Schedule effect.** The roadmap's default plan starts 2026-10-05 and reaches a
pilot-ready prototype on 2027-02-05. S0 fits in the window before that start. S1
interleaves with P1/P3 rather than preceding them, because the scanners it adds
need code on `main` to analyse. S2 runs alongside P4–P5. Net effect on the pilot
date: roughly the S2 residue plus whatever `sparc-iac` needs, not the full
5 engineer-weeks of S0–S2. Re-plan with `demo/planner.html` once the S-phases
carry real effort numbers.

---

## Cross-repository issues

Filed, not done here. Update this table as issues are opened and closed.

| # | Repository | Ask | Blocks | Issue | Status |
|---|---|---|---|---|---|
| X-1 | `dev-sec-ops-baseline` | Declare `sparc-horizon` in the org inventory and add its coverage declaration under `inputs/` | S0 exit | | Not filed |
| X-2 | `container-build-sign` | ECR repo `sparc-horizon`; add Horizon as a consumer so pin-bump issues are filed against it; confirm the Go/UI image shape fits `build-sign-publish.yml` | S1-9 | | Not filed |
| X-3 | `sparc-iac` | Emit role `SPARC_HORIZON_EMIT_ARN` scoped `risk-sentinel/*/sparc-horizon/*`; ECS Fargate service, task definition, ALB, secrets | S0-11, S2 | [#715](https://github.com/risk-sentinel/sparc-iac/issues/715) (boundary + role) | **Filed** 2026-09-19 |
| X-4 | `sparc` | `sparc-validate` rules rejecting SSP/AR/POA&M documents missing the required namespace props; publish the KSI and 800-53 mapping documents Horizon's axis swap reads; confirm the Delivery API surface Horizon consumes | P0, P1 | | Not filed |
| X-7 | estate-wide | **Evidence boundary pivot to `risk-sentinel`**, filed in unison: [sparc-iac#715](https://github.com/risk-sentinel/sparc-iac/issues/715) (hub — IAM + bucket policy + org variable), [sparc#1153](https://github.com/risk-sentinel/sparc/issues/1153), [sparc-validate#400](https://github.com/risk-sentinel/sparc-validate/issues/400) (+ its 20-repo fleet), [container-build-sign#325](https://github.com/risk-sentinel/container-build-sign/issues/325), [sparc-horizon#1](https://github.com/risk-sentinel/sparc-horizon/issues/1) | S0-11 | see left | **Filed** 2026-09-19 |
| X-5 | `sparc-validate` | Execute the ECS Fargate and secrets baselines against the deployed Horizon service and emit HDF | S2-6 | | Not filed |
| X-6 | `sparc` | Replace the illustrative namespace URI `https://risk-sentinel.org/ns/sparc` with the registered one, and register the federation namespace UUID the UUIDv5 grammar derives from | P0 | | Not filed |

---

## Open decisions

Carried from [`docs/10-risks-decisions.md`](../10-risks-decisions.md), plus the
ones this plan surfaces. All are owner decisions.

| Decision | Bearing | Needed by |
|---|---|---|
| ~~Evidence boundary~~ — **decided 2026-09-19: `risk-sentinel`.** Horizon is the first producer to use it; the estate pivots under `sparc-iac#715`. Remaining sub-decision there: whether `sparc` is subsumed by `risk-sentinel` or coexists beneath it | Which SAR the evidence joins; the prefix and the emit role scope, which must move together | Resolved |
| Signature format: CMS detached, Sigstore bundle, or JWS over canonical JSON | `internal/attest` interface | P4 |
| `go-oscal` versus types generated from the NIST JSON schemas | Whether OSCAL 1.2.x is fully covered | P0 |
| Who owns the ranking weights — each AO, or the organization | Config surface and the AO lens | P2 |
| When the ledger moves from SQLite to Postgres | S2 task definition and backup posture | P2 / S2 |
| Whether decision dates live only in SSP metadata or also come from the GRC calendar | Projection buckets | P0 |
| Public image, or ECR-only | `publish_to_dockerhub` on the S1-9 caller | S1-9 |
| Whether a GitLab mirror of the pipeline is required | Two hand-maintained copies drift; if both exist, one is declared canonical in both files | S1 |

---

## NIST control coverage (application layer)

Starts empty and grows per issue, per `issue_rules.md` step 9. The table below is
the intended shape, not a claim — no row is filled until the implementation and
its CDEF exist.

| Family | Controls Horizon is expected to implement | Status |
|---|---|---|
| AC | AC-2, AC-3, AC-6 — node-scoped authorization from `responsible-parties`, roles inheriting downward | Not started |
| AU | AU-2, AU-3, AU-9, AU-12 — the append-only hash-chained ledger | Not started |
| CM | CM-3, CM-6 — signed image, pinned digests, required checks | Not started (S1/S2) |
| IA | IA-2, IA-2(1), IA-8 — OIDC to the agency IdP, MFA delegated | Not started |
| RA | RA-5 — the scanner classes in `issue_rules.md`'s requirements table | Not started (S0/S1) |
| SA | SA-11, SA-11(1), SA-15 — SAST, SCA, the verification gate | Not started (S0/S1) |
| SC | SC-8, SC-13, SC-28 — mTLS to SPARC peers, evidence hashing, signing | Not started |
| SI | SI-7 — hash-chained ledger, signed evidence, exports carrying the chain head | Not started |
| SR | SR-3, SR-4 — SBOM, signed provenance, pinned bases | Not started (S1) |

Coverage is documented in `docs/compliance/nist-sp800-53-rev5-mapping.md` and the
OSCAL CDEFs under `docs/compliance/oscal/cdefs/`, with configuration
dependencies recorded in `remarks` — a control whose coverage depends on
`HORIZON_OIDC_ISSUER` being set is Partial without it, and saying so is the
point.
