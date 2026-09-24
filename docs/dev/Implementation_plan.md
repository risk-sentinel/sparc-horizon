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

**[`session-log.md`](session-log.md)** carries continuity between sessions: where
unpushed work stopped, and what the next session should pick up. It holds only what
GitHub cannot express, so it restates no status this file or the phase epics already
carry. Step 8 updates it in the same PR as the work.

---

## Status snapshot

> **Updated 2026-09-24.** Most values below are still the starting ones. The rows
> that have moved are P0's — the Go module, the key-grammar reference, and the
> fixture federation — and P1's first package, `internal/tree` (#74).

| Bucket | Current state |
|---|---|
| Application code | **10 packages** — `internal/canonical`, `internal/keys`, `internal/fixtures`, `internal/oscal`, `internal/project`, `internal/tree`, `internal/authz`, `cmd/genfixtures`, `cmd/oscalprobe`, `cmd/mockserver` (#38, #36, #49, #61, #74, #76, #79). No service yet; `horizon/` remains an empty placeholder and the module is rooted at the repository |
| Design docs | **11** (`docs/01`–`docs/10` + `roadmap.md`) — the design of record |
| Machine-readable contracts | **2 owned**, one of them now frozen (`api/openapi.yaml`, #61) — `api/openapi.yaml` (v0 skeleton) and `schemas/sparc-namespace-props.v1.schema.json` (v1, 9 props, namespace `https://sparc.risk-sentinel.org/ns`) — plus **1 consumed**, `sparc:lib/federation/key-grammar.v1.json`, vendored with provenance into `internal/keys/testdata/` |
| Demos | **4** static HTML files, synthetic data, seeded PRNG, no build step |
| CI workflows | **9** — `secret-scan.yml` (gate + fixture canary), `secret-scan-hdf-emit.yml` and `sonarqube-hdf-emit.yml` (emitters, both fail closed on an unset boundary), `pr-checklist.yml`, `contracts.yml` (OpenAPI, namespace schema, **fixture props**, actionlint, duplication drift), `ci.yml` (Go: fmt, vet, lint, race, coverage and package-count floors), `sonar-pr-findings.yml` (this PR's SonarCloud findings as OHDF, reported not gated — #63), `vendored-contract-freshness.yml` (scheduled; notices when SPARC's key-grammar contract moves and the vendored copy has not — #68), `action-pin-verification.yml` (scheduled and on merges touching workflows; checks each pinned SHA against the version its comment claims — #57) |
| Branch protection | **Active.** Ruleset on `main`: 7 required contexts, PR required with CODEOWNERS review, signed commits, no force-push, no deletion, bypass **pull request only**. Verified by a direct push being refused, not just by reading the config back |
| Secret scanning | **Gate + canary landed.** TruffleHog verified-only, `tests/trufflehog-fixture/` planted and asserted, exclude file scoped to the fixture alone |
| SAST / code scanning | **Sonar wired.** Project `risk-sentinel_sparc-horizon` live (private), `SONAR_TOKEN` an org secret, emitter converts to HDF and verifies the project resolves before fetching. CodeQL and `golangci-lint` still pending — Phase S1, when Go lands |
| Dependency / SBOM / SCA | **`go.mod` exists** (Go 1.25, three direct dependencies). Dependabot has no `gomod` ecosystem yet and there is no `.security/sca-allowlist.yaml` — both S1 |
| Container | **None** — the Dockerfile in [`docs/08-build-deploy.md`](../08-build-deploy.md) is a sketch, unpinned, never built |
| HDF evidence emitted | **0 artifacts.** Emitter written and fails closed on an unset boundary or absent role; blocked on the `risk-sentinel/*/sparc-horizon/*` role (`sparc-iac#715`) |
| Org inventory (`dev-sec-ops-baseline`) | **Not declared.** `devsecops-inventory-reconciliation` does not see this repo |
| `container-build-sign` consumer list | **Not listed.** No ECR repo, no signed image, no pin-bump notifications |
| AWS deployment | **None.** No `sparc-iac` module, no emit role, no task definition |
| NIST control coverage (application layer) | **0 documented.** No `docs/compliance/` tree yet |
| Highest-priority next work | **P1, and it is not blocked.** P0 is task-complete; both its remaining exit criteria are other people's — `sparc-validate` against the fixtures (`sparc#1154`, upstream) and the OpenAPI review by both owners. P1's tree builder landed as #74 and node-scoped authorization as #76, both against the OSCAL fixtures. **P1's SPARC client is blocked** on `sparc#1181` (X-13): no OSCAL document export exists under `/api/v1` for SSP, SAP, SAR or POA&M. TM-1's remaining two requirements need a signed bundle and the ledger (P2), so the next unblocked P1 work is the control-id normalisation task |

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
  against the current tree. **The first axis was then closed outright** — `docs/hud.html`
  deleted in #4, with the diff replaced by a guard against the copy returning.
- **`horizon.zip` dropped** (S0-13) and `*.zip` ignored.

- **Evidence emit proven end to end** (S0-11) on 2026-09-19. The object was read back
  from the bucket rather than inferred from a green job, which matters more here than
  usual: every emit path in the estate carries `continue-on-error: true` on the
  credential step, so a refused write leaves the workflow green *and* reports the step's
  conclusion as `success` through the API. The check that means anything is the
  `upload: … to s3://` line in the log, then the object itself.

  Landed at `risk-sentinel/2026-09-19/sparc-horizon/trufflehog/trufflehog-hdf.json` and
  its `latest/` alias, 3689 bytes each, server-side encrypted with KMS. The payload is
  the one-control execution record — `trufflehog 3.96.0`, the commit, the ref, the run
  id, `findings 0`, tagged `IA-5(7)` and `SA-11` — so a clean scan is recorded as a scan
  that ran, not as an absence.

  Two upstream fixes unblocked it, both in `sparc-iac#719`: the OIDC trust policy was
  corrected (this repository is new enough that GitHub issues it an **immutable
  subject**, `repo:<org>@<id>/<repo>@<id>:…`, which the original pattern did not match),
  and the encryption deny was reverted. That deny returns under `sparc-iac#721` — see
  #10, resolved on this branch, for why the emit would otherwise have started failing
  again.

**Phase S0 is complete.** The last two items closed on 2026-09-19: the compliance directory (#11) and the threat model attestation (#12). The threat model produced eight findings, none marked mitigated — which is the right result for a repository with no application code — and they are carried as named requirements on P1, P2, P4, P6, P7 and S1 rather than as a document nobody reads again. TM-4 goes to `docs/10-risks-decisions.md` as an open phase-0 decision; TM-6 is filed as `sparc#1159`. The attestation was then **re-issued the same day** (#28): #27 fixed what a signature is computed over, which tripped the attestation's own early-staleness trigger for a change to the canonicalisation rule, and produced TM-9. That is the freshness mechanism working on the first occasion it fired, and the reason to prefer a dated attestation over a page that ages quietly. It then fired a **second** time hours later, on #30's normative UUID key grammar — logged rather than re-attested at the time, so r2 read as stale in that area for a day. **#31 settled the cadence and produced r3 on 2026-09-20**, and **the P0-exit checkpoint it promised produced r4 on 2026-09-22** (#64), folding in five firings and adding TM-10 and TM-11 — both measured rather than supposed, and neither reachable by reasoning. That checkpoint also found the interim ledger had not been maintained, which is recorded in the ledger itself rather than corrected quietly. The cadence rule was: while a phase whose deliverables are the contracts the model reasons about is open, a firing is logged in the model's interim staleness ledger and folded in at phase exit, rather than re-attested per merge. The triggers are **not** narrowed, a firing that contradicts a finding re-issues immediately, and the expiry does not reset on a revision. The next checkpoint is **P1 exit**, or any firing that contradicts a finding, which re-issues immediately.

**S0-12 branch protection is active** as of 2026-09-19. Ruleset `main`, enforcement
`active`, copied from `sparc-validate`'s shape with two deliberate departures:

- **bypass `pull_request`, not `always`.** `sparc-validate` grants repository admins
  `always`, which permits a direct push to the protected branch. `pull_request` refuses
  that while still letting the owner merge their own PR — which matters where one person
  authors most changes and self-approval is impossible. Some web UIs will not render the
  bypass-merge button under this mode; the CLI honours it.
- **`required_signatures` added.** Not present on `sparc-validate`. Safe here because
  every commit on `main` already verifies server-side, checked before enabling — turning
  it on against unsigned history would have blocked all work.

Verified by attempting a direct push to `main` and reading the refusal
(`GH013 ... Changes must be made through a pull request`, `7 of 7 required status checks
are expected`), not by reading the configuration back. A protection rule nobody has
tested is a claim.

`delete_branch_on_merge` is now on, matching `sparc-validate`, and the merged
`feature/1` and `feature/4` branches were removed.

**Neither HDF emitter is a required context, deliberately.** Both run on push-to-main
and schedule but never on `pull_request`, so requiring either would leave every PR
waiting on a check that cannot arrive.

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
| S0-9 | `sonarqube-hdf-emit.yml` copied in self-contained (a public repo cannot call a private reusable), CONFIGURATION block set to `REPO_SLUG: sparc-horizon`. It verifies the project **resolves** before fetching, so it fails rather than reporting a clean empty result | [#6](https://github.com/risk-sentinel/sparc-horizon/issues/6), [#13](https://github.com/risk-sentinel/sparc-horizon/issues/13) | 2026-09-19 |
| S0-10 | `.github/dependabot.yml` for `github-actions` now; `gomod` and `npm` ecosystems added in S1 when the manifests exist | | 2026-09-19 |
| S0-11 | Evidence path proven end to end with the secrets HDF: prefix `risk-sentinel/<date\|latest>/sparc-horizon/trufflehog/`, provenance stamped, and the **landed object read back and verified** rather than trusting a green upload. No `\|\| 'sparc'` fallback — fail closed on an unset boundary | [#1](https://github.com/risk-sentinel/sparc-horizon/issues/1), [#10](https://github.com/risk-sentinel/sparc-horizon/issues/10) | 2026-09-19 |
| S0-12 | Branch protection: ruleset copied from `sparc-validate`, `strict_required_status_checks_policy: true`, signed commits, reviews required, bypass **pull request only**, enforcement **active** not evaluate. Required contexts are the seven names below, all now observed reporting | | 2026-09-19 |
| S0-13 | Delete `horizon.zip` from history-going-forward and gitignore it; it is a snapshot of the repo that goes stale on every commit *(already dropped from the working tree 2026-09-19 — confirm the ignore rule)* | | 2026-09-19 |
| S0-14 | De-duplicate `docs/hud.html` (byte-identical to `demo/hud.html`) — removed, with a CI guard against it returning | [#4](https://github.com/risk-sentinel/sparc-horizon/issues/4) | 2026-09-19 |
| S0-15 | `docs/compliance/` skeleton: `README.md`, `nist-sp800-53-rev5-mapping.md`, `oscal/cdefs/`, and the inline-control-comment format. Horizon's own control story starts empty and grows per issue, per `issue_rules.md` step 9 | [#11](https://github.com/risk-sentinel/sparc-horizon/issues/11) | 2026-09-19 |
| S0-16 | `docs/dev/Developer_Collision_Avoidance_Plan.md` — domain ownership and hot files | | 2026-09-19 |
| S0-17 | Threat model and security architecture review recorded as a dated, signed attestation document. This is the one stage `dev-sec-ops-baseline` deliberately does not automate: it produces a document and a conversation, and is evidenced through the attestation path where freshness is asserted | [#12](https://github.com/risk-sentinel/sparc-horizon/issues/12) | 2026-09-19 |
| S0-18 | **Canary for actionlint's shellcheck integration.** `actionlint` exits 0 when the `shellcheck` binary is absent — it skips the integration rather than reporting it, so a runner-image change would remove a class of coverage while the job stayed green. `tests/actionlint-fixture/` carries a planted `SC2012` defect the lint job must report | [#8](https://github.com/risk-sentinel/sparc-horizon/issues/8) | 2026-09-19 |

### Required check contexts, as the forge reports them

Observed reporting on PR #2, 2026-09-19 — all seven green. These are the strings to
put in the ruleset. They are **read from the forge, not from the workflow files**,
because the reported name is the job's `name:` rather than the workflow's, and a
mismatch produces a required check that never arrives:

```text
Duplicated copies agree
Fixture detection (proves scanner works)
Namespace schema
OpenAPI lint
Test plan checklist
Verified secrets gate
Workflow lint
```

`Secret scan HDF emit (TruffleHog)` is deliberately **not** in that list: it does not
run on `pull_request`, so requiring it would block every PR permanently.

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
- **Depends on:** S0. **S1-1 and S1-2 landed early, in P0** (#38) — the fixture
  generator cannot derive a UUID without implementing the key grammar, so the
  first Go package arrived in P0 and its obligations came with it rather than
  being deferred. The remaining twelve tasks still depend on P1's packages
- **Interleaves with:** P1

| ID | Task | Issue | Done |
|---|---|---|---|
| S1-1 | `go.mod` at the Go version pinned in [`docs/08-build-deploy.md`](../08-build-deploy.md), `.golangci.yml` with a pinned `golangci-lint`, `gosec` enabled | [#38](https://github.com/risk-sentinel/sparc-horizon/issues/38) | 2026-09-20 |
| S1-2 | `ci.yml`: `gofmt -l`, `go vet`, `golangci-lint`, `go test ./... -race` with coverage. Assert the package count and coverage number — a package with no tests exits 0 and prints `no test files` | [#38](https://github.com/risk-sentinel/sparc-horizon/issues/38) | 2026-09-20 |
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
| P0 | Contracts and fixtures | 3 ew | **S0** | Namespace schema v1 already exists. **Landed:** the UUIDv5 key grammar (#30, #37), the `go-oscal` decision (#26) and its re-test (#49), the Go toolchain and CI (#38), and the Go reference implementation, test vectors and fixture federation (#36). Frozen API v0 and the mock over the fixture federation (#61). **All P0 tasks are complete**; the exit criteria are what remain, and one of them is upstream. The `sparc-validate` rules are filed as `sparc#1154` |
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

**All cross-repo dependencies are filed.** Re-verified 2026-09-22 against the live issue
state rather than against this table.

That re-verification found one gap, which is worth recording as a pattern rather than a
correction. `sparc-iac#715` carried **two** asks — the evidence boundary and the ECS Fargate
deployment — and closed when the first was delivered and proven. The second went with it, and
nothing tracked S2-3 for three days. Nothing was blocked, because Horizon has no image to deploy
yet; the defect is that this table asserted completeness while an ask had quietly stopped
existing. **A closed issue that carried more than one ask is the case to check**, because the
closure is legitimate and the remainder is invisible.

| # | Repository | Ask | Blocks | Issue | Status |
|---|---|---|---|---|---|
| X-1 | `dev-sec-ops-baseline` | Declare `sparc-horizon` in the org inventory and add its coverage declaration under `inputs/`. **Currently an undeclared repository, so `devsecops-inventory-reconciliation` is failing on it today** | S0 exit | [#71](https://github.com/risk-sentinel/dev-sec-ops-baseline/issues/71) | **Filed** 2026-09-19 |
| X-2 | `container-build-sign` | ECR repo `sparc-horizon`; add Horizon as a consumer so pin-bump issues are filed against it; confirm the Go/UI image shape fits `build-sign-publish.yml` — Horizon is the framework's **first application image** | S1-9 | [#326](https://github.com/risk-sentinel/container-build-sign/issues/326) | **Filed** 2026-09-19 |
| X-3 | `sparc-iac` | Emit role `SPARC_HORIZON_EMIT_ARN` scoped `risk-sentinel/*/sparc-horizon/*` | S0-11 | [#715](https://github.com/risk-sentinel/sparc-iac/issues/715) | **Delivered and proven** 2026-09-19 — the first object was read back from the bucket. Required a trust-policy correction for GitHub's immutable subject form (`sparc-iac#719`). Encryption deny returns landed as `sparc-iac#721` |
| X-3a | `sparc-iac` | **ECS Fargate service, task definition, ALB, secrets wiring** for the `HORIZON_*` contract in `docs/08-build-deploy.md`, plus a task role for evidence access distinct from CI's emit role. The signing key wants a platform key service rather than an environment variable — threat model TM-5 shapes the task definition | S2-3 | [`sparc-iac#753`](https://github.com/risk-sentinel/sparc-iac/issues/753) | **Filed 2026-09-22.** Split out of #715, which closed carrying it. Not blocking: Horizon has no image to deploy until S1 |
| X-4 | `sparc` | `sparc-validate` rules rejecting SSP/AR/POA&M documents missing the required namespace props; publish the KSI and 800-53 mapping documents Horizon's axis swap reads; confirm the Delivery API surface Horizon consumes | P0, P1 | [#1154](https://github.com/risk-sentinel/sparc/issues/1154) | **Filed** 2026-09-19 |
| X-7 | estate-wide | **Evidence boundary pivot to `risk-sentinel`**, filed in unison: [sparc-iac#715](https://github.com/risk-sentinel/sparc-iac/issues/715) (hub — IAM + bucket policy + org variable), [sparc#1153](https://github.com/risk-sentinel/sparc/issues/1153), [sparc-validate#400](https://github.com/risk-sentinel/sparc-validate/issues/400) (+ its 20-repo fleet), [container-build-sign#325](https://github.com/risk-sentinel/container-build-sign/issues/325), [sparc-horizon#1](https://github.com/risk-sentinel/sparc-horizon/issues/1) | S0-11 | see left | **Horizon's leg complete** 2026-09-19 — first producer in the estate writing under `risk-sentinel`, verified from the bucket. The flip was applied before the bucket policy accepted the new prefix, which denied every producer estate-wide while their workflows stayed green (`sparc-iac#719`); the encryption deny returns under `sparc-iac#721`, which #10 prepares for. The rest of the estate's legs are not Horizon's to close |
| X-5 | `sparc-validate` | Execute the ECS Fargate and secrets baselines against the deployed Horizon service and emit HDF | S2-6 | [#401](https://github.com/risk-sentinel/sparc-validate/issues/401) | **Filed** 2026-09-19 |
| X-6 | `sparc` | Replace the illustrative namespace URI with the registered one, and register the federation namespace UUID the UUIDv5 grammar derives from | P0 | [#1155](https://github.com/risk-sentinel/sparc/issues/1155) | **Delivered 2026-09-21, adopted here in #54.** SPARC had already registered `https://sparc.risk-sentinel.org/ns` (its own, not Horizon's placeholder), so the change belonged here; the federation namespace UUID `9f434272-f796-589b-b972-954790395630` is **derived** from that URI rather than random, so any peer recomputes it. Every fixture identifier regenerated |
| X-8 | `sparc` | **Unblock the 800-53 attribution for the inherited AWS platform layer.** Horizon's component definition imports the AWS Labs service definitions for component identity; their control implementations are keyed to AWS Security Hub control ids, and SPARC already owns the Security Hub to NIST 800-53 rev 5 converter. The blocker is that those definitions import as a document with **zero controls**, so the converter has nothing to map | S0-15 | [#1103](https://github.com/risk-sentinel/sparc/issues/1103) | **Open upstream**, filed 2026-09-03 before Horizon needed it. Recorded here 2026-09-19 because the inherited rows in `docs/compliance/nist-sp800-53-rev5-mapping.md` now depend on it. Horizon must not work around it by authoring the crosswalk locally |
| X-9 | `sparc` | **Deduplication must be scoped by originating party, not by object UUID alone.** UUIDv5 over natural keys is deterministic and its grammar is published, so any peer can compute any boundary's identifiers and claim them. `sparc` owns the federation trust fabric and therefore the dedup semantics | P6, threat model TM-6 | [#1159](https://github.com/risk-sentinel/sparc/issues/1159) | **Filed** 2026-09-19. Horizon scopes its own ingestion on (UUID, originating party) regardless, but cannot fix the fabric's semantics from here |
| X-10 | `sparc` | **Ruby and Python reference implementations of the UUIDv5 key grammar, with shared test vectors.** Horizon specified the grammar normatively and owns the Go reference; Ruby and Python are SPARC's runtimes | P0 | [#1161](https://github.com/risk-sentinel/sparc/issues/1161) | **Delivered 2026-09-21.** Both ports shipped, and **ownership of the shared artifact moved upstream**: `sparc:lib/federation/key-grammar.v1.json` is the source of truth for the field lists, type rules, vectors and dedup rule, and Horizon consumes it (#54). Our `fixtures/key-vectors.v1.json` is deleted. Conformance holds on all **28** vectors and the nine field lists. The one type rule that was not adopted — `family-id` normalisation — was challenged as `sparc#1175` and **fixed upstream in Horizon's favour**; re-vendored in #68 |
| X-11 | `sparc` | **Control identifiers are canonical in the catalog endpoints and raw everywhere else.** `ControlId` (SPARC #852) defines `ac-2.1` as canonical because `control-id` is `TokenDatatype` in six OSCAL schemas, but it reaches only `ksi_catalog` and `control_lookups`. The endpoints that carry identifiers *between* documents do not use it: `evidences` builds control links from raw caller strings, `ssp_control_statements` filters on exact string match, and `catalog_controls.control_id` has a case-sensitive unique index with no normalisation on write | P0, P1 | [#1162](https://github.com/risk-sentinel/sparc/issues/1162) | **Filed** 2026-09-20. Framed as SPARC's own OSCAL conformance, with no Horizon coupling. Horizon canonicalises on ingest regardless — `internal/canonical` accepts every observed spelling and converges them — so this is defensive on both sides rather than a dependency |
| X-12 | `sparc` | **`key-grammar.v1.json` lowercases `family-id` unconditionally**, so a family from a non-NIST authority — `ACM`, `S3`, `EC2` — is normalised into a value that names nothing, exactly as `ACM.1` → `acm.1` would. Horizon holds the vocabulary-scoped rule #37 established, so the same input derives **two different cell identifiers** with neither side erroring | P2 (projection cells) | [`sparc#1175`](https://github.com/risk-sentinel/sparc/issues/1175) | **Filed 2026-09-22. ACCEPTED and merged upstream 2026-09-23 as `1cb999b1`** — `vocabulary-normalisers` is now keyed by vocabulary **and** identifier kind, and the `family-id` type rule is vocabulary-aware. Two new vectors carry `b9691843-…` and `f2a38fbf-…`, exactly what Horizon derived, so the divergence is closed rather than re-specified. **No UUID moved.** Re-vendored here in #68, which also added `vendored-contract-freshness.yml` because the conformance test asserts against the vendored copy and could not have noticed |
| X-13 | `sparc` | **OSCAL export is reachable over `/api/v1` for `cdef_documents` only.** `GET /api/v1/cdef_documents/:slug/export?format=oscal` works, via `OSCAL_EXPORT_FORMATS` and a `validate` flag. The equivalent SSP, SAP, SAR and POA&M endpoints have no `format` branch — they return `JsonExportService`'s internal shape (`{document_name, controls}` for an SSP: no metadata, no parties, no back-matter). The OSCAL exporters all exist and are used by `download_oscal` on the **web** routes, which sit behind session auth with mandatory FIDO2 and OIDC/PIV, so a service account cannot reach them. Party UUIDs are constructed *during* the export, not stored, so they cannot be assembled from other API responses either | **P1** — the SPARC client has no OSCAL to parse without it | [`sparc#1181`](https://github.com/risk-sentinel/sparc/issues/1181) | **Filed 2026-09-23.** Narrow by design: extend the pattern `cdef_documents` already proves, rather than build anything. `sparc#895` (Catalog API had no `Api::V1` surface) is the accepted precedent. Measured in #70 against `origin/main` `bb82c75f`. **Horizon is not blocked meanwhile** — `internal/tree` and `internal/authz` build against the OSCAL fixtures, which carry the parties, `member-of-organizations` and `responsible-parties` the tree joins on |

---

## Open decisions

Carried from [`docs/10-risks-decisions.md`](../10-risks-decisions.md), plus the
ones this plan surfaces. All are owner decisions.

| Decision | Bearing | Needed by |
|---|---|---|
| ~~Evidence boundary~~ — **decided 2026-09-19: `risk-sentinel`.** Horizon is the first producer to use it; the estate pivots under `sparc-iac#715`. Remaining sub-decision there: whether `sparc` is subsumed by `risk-sentinel` or coexists beneath it | Which SAR the evidence joins; the prefix and the emit role scope, which must move together | Resolved |
| Signature format: CMS detached, Sigstore bundle, or JWS over canonical JSON | `internal/attest` interface. Whichever is chosen, it signs the received bytes — see #26 | P4 |
| ~~`go-oscal` versus types generated from the NIST JSON schemas~~ — **decided 2026-09-19: `go-oscal`, pinned** (#26). 1.2.x coverage is real; the round trip is lossless apart from timestamp normalisation; the fallback was the same generator self-hosted | Settled. It surfaced a new rule instead: signatures cover the bytes as received, never a re-serialisation | Resolved |
| Who owns the ranking weights — each AO, or the organization | Config surface and the AO lens | P2 |
| When the ledger moves from SQLite to Postgres | S2 task definition and backup posture | P2 / S2 |
| Whether decision dates live only in SSP metadata or also come from the GRC calendar | Projection buckets | P0 |
| ~~Attestation cadence while contracts are still being written~~ — **decided 2026-09-20: checkpoint at phase exit** (#31, r3). Triggers unchanged; interim firings logged in the threat model's staleness ledger; a firing that contradicts a finding re-issues at once; the expiry does not reset on a revision | How often the threat model is re-reviewed, and whether SA-11(2) may be read as continuously current | Resolved for P0; #31 stays open until the P0-exit checkpoint |
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
