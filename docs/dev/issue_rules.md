# SPARC-Horizon Issue Process Rules

Standard workflow for every issue in the sparc-horizon repository.
These rules are **mandatory** — no exceptions without explicit owner approval.

Horizon is a new repository. It inherits the estate's process wholesale rather
than growing its own: the rules below are ported from `risk-sentinel/sparc`,
`sparc-validate`, and `container-build-sign`, with the Ruby/Rails specifics
replaced by Go and TypeScript equivalents. Where a rule cites an incident, the
incident happened in the named repository — Horizon has no history yet, and
inventing one would make the rule easier to dismiss.

**Security before features.** The scanning pipeline, evidence emission, and
branch protection described here land before application code does. See
[`Implementation_plan.md`](Implementation_plan.md) Phase S0. A prototype that
reaches AWS without them is not a prototype of the thing we intend to ship.

---

## Hard Guardrails

- **Never push directly to `main`** — all changes go through feature/bug branches
- **Never merge PRs** — only the repository owner merges
- **Always plan before implementing** — step 5 is not optional
- **Never modify CI workflows without explicit approval**
- **Never suppress a scanner finding without explicit approval** — see
  [Suppressing scanner findings](#suppressing-scanner-findings)
- **Never use `brew`** — find alternatives or ask the owner
- **Never write to another repository.** Horizon reads the rest of the estate;
  it changes nothing in it. Work that belongs elsewhere is **filed as an issue
  in that repository** and picked up by its owner. See
  [Cross-repository work](#cross-repository-work)

- **No AI-assistant attribution anywhere in the repository or its artifacts.**
  PR titles and bodies, commit messages, issue and PR comments, code comments and
  documentation are written in the project's own voice. Do not name Claude,
  Claude Code, Anthropic or any assistant, and do not emit `Generated with …` or
  `Co-Authored-By: <assistant>` trailers. **Tooling that appends these by default
  must be overridden — a tool default is not an exception to this rule.** The
  history is read as the change-control record for an authorization boundary, so
  a commit describes *what changed and why*, not what produced it. If an
  attribution line reaches a PR body, edit it out; GitHub allows editing the body
  of a merged PR. The same rule is binding in `sparc`, `sparc-iac`, and
  `container-build-sign`, so the repos read consistently to an assessor.

- **If the current branch is not `main`, ASK before switching branches or cutting
  a new one.** Run `git rev-parse --abbrev-ref HEAD` before any `git checkout` /
  `git switch` / `git checkout -b`. Not on `main` → stop and ask. There is no
  exception for "this new work is unrelated and deserves its own branch": a
  second branch cut from `main` gets a **stale copy of every shared file** — the
  roadmap, `issue_rules.md`, `CLAUDE.md` — because the current bundle's edits
  live on the unmerged branch, and an edit made against that stale copy has to be
  re-done rather than cherry-picked. When new work arrives mid-bundle the default
  is to do it **on the current branch**; if it genuinely warrants separation, say
  so and ask.

- **Never commit real secrets; keep test credentials scanner-clean.** Real
  secrets live only in gitignored `.env` files, the CI secret store, or AWS
  Secrets Manager — never in tracked code, compose files, or workflows. Horizon
  handles signing keys and an OIDC client secret, so this is not theoretical.
  Test/local/CI credentials MUST be obvious throwaways (e.g.
  `postgres:password@`, `Initial-Pwd-1234`). When the secret scanner flags one,
  clear it by scoping the exclusion to that path in
  `.trufflehog-exclude-paths` — never by widening the scan or lowering its mode.
  The deliberately-planted fixture under `tests/trufflehog-fixture/` is the one
  exception, and it exists to be detected.

- **Never put account-specific identifiers in commit / PR / issue text** — no
  account numbers, ARNs, ECR/registry URIs (they embed the account number),
  resource IDs (`sg-…`, `vol-…`, `db-…`), or regions in narrative text.
  Reference abstractly ("the emit role", "the prod account") or by variable name
  (`SPARC_HORIZON_EMIT_ARN`, `ECR_REGISTRY`). Git history and GitHub text are
  lower-trust surfaces than the code. Config that legitimately needs a literal
  value is a separate concern.

---

## Cross-repository work

Horizon depends on five sibling repositories and writes to none of them. When
work belongs to one of them, **file an issue there**, link it from the Horizon
issue, and record it in the cross-repo table in
[`Implementation_plan.md`](Implementation_plan.md). Do not edit a sibling
checkout, and do not work around a missing upstream change locally without
saying so in the PR body.

| Repository | Owns | Horizon files an issue there when |
|---|---|---|
| `sparc` | Authoritative catalogs, baselines, profile resolution, crosswalks, the Delivery API, federation trust fabric | A namespace prop, a `sparc-validate` rule, a mapping document, or a Delivery API endpoint Horizon consumes needs to change |
| `sparc-validate` | InSpec/CINC profiles, HDF production, overlay pins | A profile must assert something about Horizon's deployed resources, or an HDF conversion route is missing |
| `container-build-sign` | Image build / scan / SBOM / sign / publish; the shared reusable scan workflows | Horizon needs an ECR repo, a caller added to the consumer list, a base-image bump, or a change to a reusable workflow it calls |
| `sparc-iac` | All AWS Terraform, IAM roles, ECR repos, ECS services, evidence-bucket policy | Horizon needs its deployment, its emit role, its task definition, or a secret provisioned |
| `dev-sec-ops-baseline` | The SDLC evidence profile and the org repository inventory | Horizon must be declared in the inventory, or a coverage declaration needs adding |

**Horizon holds no Terraform.** `docs/08-build-deploy.md` sketches a
`deploy/terraform/` and `deploy/helm/` in-repo; that sketch predates this
repository joining the estate and is superseded. AWS infrastructure for the
estate lives in `sparc-iac`, and Kubernetes is not a target for the prototype —
the deployment model is **ECS Fargate in AWS commercial**, matching the existing
SPARC deployment and `rs-aws-ecs-fargate-baseline`. Horizon owns the container
and the configuration contract (`docs/08-build-deploy.md`'s env table); the
service that runs it is a `sparc-iac` issue.

---

## Suppressing scanner findings

**Every suppression requires explicit owner approval, before it is committed.**

A suppression is a decision that a reported risk will not be fixed. That is a
risk-acceptance decision, and risk acceptance belongs to the owner — not to
whoever happened to be looking at a red check. The person suppressing is always
the person most motivated to make the red thing go away, which is exactly why
they should not be the one deciding.

This rule is estate-wide (`sparc-validate/docs/dev/issue_rules.md` carries the
canonical text). It applies to **any scanner** in **any repository**; only the
syntax changes.

### What counts as a suppression

Not exhaustive. If the effect is *a finding stops being reported*, the rule
applies.

| Scanner | Mechanism |
|---|---|
| SonarCloud / SonarQube | `// NOSONAR`, "Won't fix" / "False positive" in the UI, `sonar.issue.ignore.*`, `sonar.exclusions` |
| CodeQL / code scanning | Dismissing an alert, `paths-ignore`, query filters |
| `golangci-lint` | `//nolint:<linter>`, disabling a linter in `.golangci.yml`, an `issues.exclude-rules` entry |
| `gosec` | `//nolint:gosec`, `#nosec` (with or without a rule ID), `-exclude=` |
| `go vet` / `staticcheck` | `//lint:ignore`, removing a check from the enabled set |
| `govulncheck` | Excluding a module, or pinning to a version to make the report quiet rather than to fix the vulnerability |
| Grype / Trivy | `.security/sca-allowlist.yaml` entries, `.trivyignore`, raising `--severity` to hide a class |
| TruffleHog | Dismissing an alert, an entry in `.trufflehog-exclude-paths`, dropping `--only-verified` coverage |
| Dependabot | Dismissing an alert, `ignore:` in `dependabot.yml` |
| ESLint / tsc | `// eslint-disable`, `@ts-ignore`, `@ts-expect-error`, loosening `strict` |
| axe / accessibility | Excluding a rule or a region from the scan |
| Any CI gate | `continue-on-error: true` on a check that was gating, removing a required status check, lowering a quality-gate threshold |

**Deleting or narrowing a test so it stops failing is a suppression too**, even
though no scanner is involved. So is `t.Skip()` added to make a red suite green.

### The bar for approving one

Suppression is legitimate. It is the *unexamined* suppression that is not. A
request should carry:

1. **What the finding says**, quoted — rule ID, severity, file and line.
2. **Why it is wrong or accepted.** "False positive" is a claim, not a reason;
   say what the analyser cannot see. If it is a real risk being accepted, say
   what compensates for it.
3. **What was tried first.** Fixing the code is the default. Reaching for a
   suppression before attempting a fix is the failure mode this rule exists to
   catch.
4. **Scope and lifetime.** One line, or the whole package? Permanent, or until a
   named issue closes?

If you believe a rule cannot be satisfied, **demonstrate it** — attempt the fix
and show the failure, against the pinned toolchain. An untested "this would
break" is not a reason. (`dev-sec-ops-baseline` PR #23 is the estate's worked
example of both outcomes: one finding fixed because the analyser was right, one
suppressed because the constraint was verified against the pinned image rather
than asserted from memory.)

### How to record one

- **Prefer in-code suppression over the scanner's UI.** A `//nolint:gosec` with
  the reason beside it travels with the file, survives a project re-key, and is
  visible in review. A "won't fix" clicked in a dashboard is invisible to anyone
  reading the code and is lost when the project is recreated.
- **The reason goes next to the suppression**, not only in the commit message.
  A bare `//nolint` with no rule ID and no reason is rejected in review.
- **Platform alert dismissals are the exception** — dismiss those in the platform
  *with a reason*, because the platform records who decided and when, which an
  ignore-file cannot. `dev-sec-ops-baseline`'s `devsecops-dismissals-accountable`
  control asserts that owner and reason are present and carries the disposition
  into the HDF as Not Applicable rather than dropping the finding.
- **CVE dispositions go in the tracked allow-list**, with `rationale`,
  `nist_control`, `reviewed_by`, and `next_review_date` — never as an inline
  skip. An inline skip makes the scanner report the finding `SKIPPED`, which
  removes it from POA&M generation, review cadence, and assessor visibility
  entirely (`sparc-iac`'s standing rule for `checkov-baseline.yml`).
- **Never suppress silently in a large PR.** Call it out in the PR body so it is
  reviewed as a decision rather than skimmed as noise.

### Why this is stricter here than elsewhere

These repositories produce FedRAMP evidence, and Horizon's whole claim is that a
cell recomputes from the evidence. A suppressed finding does not just disappear
from a dashboard — it changes what the evidence package asserts, and therefore
what Horizon projects. An assessor asking "who accepted this, and on what basis"
needs an answer that is not "a scanner was quiet that day".

---

## Workflow Steps

1. **Pull from Main** unless otherwise noted
2. **Assign the issue** to me
3. **Review the issue** and updated notes/comments
4. **Start a fresh branch** — `feature/` or `bug/` prefix with the issue number
   in the branch name (e.g. `feature/12_secret_scan_pipeline`)
5. **Create a plan** — get approval before writing code
6. **Implement the approved plan**
7. **Troubleshoot any issues**
8. **Update project documentation:**
   - `docs/dev/Implementation_plan.md` — mark the issue complete, update phase
     status, include Started/Completed dates, and update the cross-repo issue
     table if anything was filed
   - `docs/dev/Developer_Collision_Avoidance_Plan.md` — file lists and status
   - **`docs/compliance/threat-model.md`'s interim staleness ledger — if the change trips an
     early-staleness trigger.** One row: the firing, which trigger, whether it contradicted a
     finding, and the disposition. This is cheap in the PR that caused it and expensive at a
     checkpoint: five firings went unlogged between r3 and the P0-exit checkpoint and had to be
     reconstructed from the git log, which is the work the ledger exists to avoid. **An empty
     ledger and an unmaintained one read identically**, so a checkpoint reconstructs from the log
     regardless rather than trusting the table
   - `docs/dev/session-log.md` — a new entry at the top: where unpushed work
     stopped, alternatives rejected that reached no PR body or issue, upstream
     blocker checks with the date checked, and the next slice. Only what GitHub
     cannot express — not task status, not gate measurements. The file's own
     header states the rule. It updates **in this PR**, not out of band: a
     continuity record maintained separately goes stale unnoticed, which is the
     failure it exists to prevent
   - The design docs (`docs/01`–`docs/10`, `docs/roadmap.md`) when the issue
     changes a contract they describe. These docs are the design of record for
     the prototype; code that contradicts them is a docs bug or a code bug, never
     an accepted divergence
   - `api/openapi.yaml` and `schemas/sparc-namespace-props.v1.schema.json` when
     the API or namespace surface changes. Namespace changes within v1 are
     **additive only**
   - `demo/` when a design doc it mirrors changes — `demo/full-plan.html`
     contains verbatim copies of both demo scripts and the prose of `docs/01`–
     `docs/10`. The demos live in `demo/` only; do not copy one into `docs/`
   - **Carry deferred mandatory updates forward.** If a required update cannot
     land in the current PR, it MUST be added to the scope of the next related
     PR and tracked; never silently dropped. The next PR's description calls out
     the carried-over item
9. **Compliance artifact review** — if the issue touches security-critical code
   (authentication, authorization, audit, session management, crypto, signing,
   input validation, or configuration), update:
   - `docs/compliance/nist-sp800-53-rev5-mapping.md` — control status,
     implementation summary, and code locations for affected controls
   - `docs/compliance/oscal/cdefs/*.json` — OSCAL component definitions for
     new/changed control implementations, with configuration dependencies in
     `remarks`
   - Inline NIST control comments in modified source files
   - **Goal:** maximize documented application-layer control coverage. Horizon
     asserts other systems' posture; its own has to hold up to the same read
10. **Run the verification gate** — see [Verification gate](#verification-gate).
    Not a subset of it, and not in a different order
11. **Commit / push changes**
    - Reference the issue in all commit messages
    - No assistant attribution in the message — see the guardrail above
12. **Wait for user testing** — functional testing, regression report review
13. **Create a PR**
    - Reference the issue so it auto-closes on merge
    - Five-section body: `## Summary`, `## Changes`, `## Test plan`,
      `## Verified by CI`, `## Post-merge verification`, `## Notes`. Checkboxes
      belong in **Test plan only** — `pr-checklist.yml` fails on any unchecked
      `- [ ]` elsewhere in the body
    - Record what was measured, not that it passed: counts, versions, and the
      commands that produced them
    - **Check the body for assistant attribution before opening it** — this is
      the surface it slips through on, because the trailer is appended after the
      body is written
    - Wait for the owner to merge before moving forward

---

## Verification gate

Run every stage **serially**, in this order. Do not parallelise, do not
substitute a subset. Each stage records what it measured.

```bash
# 1. Format and vet — fast, catches the trivial before the slow stages run
gofmt -l . ; go vet ./...

# 2. Lint (pinned version; see .golangci.yml)
golangci-lint run ./...

# 3. Unit + race + coverage. Full suite before every push, not targeted tests
go test ./... -race -covermode=atomic -coverprofile=coverage.out > /tmp/gotest.log 2>&1; RC=$?
go tool cover -func=coverage.out | tail -1

# 4. Go vulnerability check against the module graph
govulncheck ./... > /tmp/govulncheck.log 2>&1; RC=$?

# 5. Web UI: types, lint, unit, accessibility
(cd web && npm ci && npm run typecheck && npm run lint && npm test)

# 6. Container smoke — the image that SHIPS, built the way CI builds it
docker build -t sparc-horizon:dev .
docker run --rm -e HORIZON_DB_DSN=sqlite:///tmp/h.db -p 8080:8080 sparc-horizon:dev &
# then: /healthz, /v1/tree against the fixture federation, and the Playwright
# smoke suite (Chrome), with zero CSP violations

# 7. Contract checks
npx @redocly/cli lint api/openapi.yaml
npx ajv-cli compile -s schemas/sparc-namespace-props.v1.schema.json --spec=draft2020
```

### Reading this pull request's SonarCloud findings

`sonar-pr-findings.yml` fetches the findings for **this PR** as OHDF and writes them to the job
summary, so they are readable in the PR's own checks. The full OHDF is an artifact:

```bash
gh run download <run-id>            # the run of "SonarCloud PR findings"
```

Read it rather than the dashboard. The quality gate fails on the Security Rating condition and
little else this project hits, so **maintainability findings pass the gate** — they are reported
here and nowhere else in the repository.

It reports; it does not gate. A PR with no findings passes, and the count is never asserted:
asserting it would make the job fail on good news. What *is* asserted is that an analysis of this
PR's head commit existed before the fetch, because an absent analysis returns an empty result that
converts into a clean-looking report.

**A clean PR produces no artifact**, only the summary line saying so. `hdf fetch sonarqube` exits 1
on a zero-issue result, so the fetch is skipped when SonarCloud reports nothing to convert — an
upstream defect, written up in [`hdf-cli-empty-sonarqube-result.md`](hdf-cli-empty-sonarqube-result.md).
The count that decides this comes from `issues/search` directly, which also gives the job a second,
independent source for the number: if the API reports findings and the OHDF carries none, the job
fails rather than reporting clean.

The **evidence** emit (`sonarqube-hdf-emit.yml`) is a different thing and stays `main`-only. PR
findings are a development signal; evidence is what landed on the default branch.

### Targeted runs during development are fine; the full suite gates the push

A single test while iterating:

```bash
go test ./internal/project -run TestStateAt -v
go test ./internal/project -run 'TestStateAt/expired_observation' -v
```

### Measurement rules — how results get faked

These are the estate's rules, and they are in the gate because each one has
produced a false green somewhere in it.

- **Capture each command's own `rc` immediately**: `cmd > log 2>&1; RC=$?`. A
  trailing `grep` returns 1 when it finds nothing, turning a PASS into "exit 1";
  a trailing `tail` returns 0, turning a FAIL into "exit 0". Check the status of
  the command you care about, not the last element of a pipeline.
- **A no-output command is a FAILED measurement**, not a pass. Redirect the full
  log to a file and read it.
- **Read the summary line**, not a character count over the log. Counting `F`s
  or `FAIL`s across a whole log counts prose and reports failures that do not
  exist.
- **`go test` with no matching tests exits 0 and prints `no test files`.** A
  package with no tests and a package whose tests all pass are the same exit
  code. Assert the coverage number and the package count, not the exit status.
- **`go test ./...` skips packages that fail to build only if you let it.** A
  build failure in one package must fail the stage; read the log for
  `[build failed]`.
- **Read skip reasons, not the pass count.** A suite that skipped everything for
  want of a fixture reports no failures while executing nothing.
- **Verify the change is in the built image**, not merely that the image built.
- **`govulncheck` needs the module graph**, so it reports nothing useful if run
  outside the module or against a vendored tree it cannot resolve. Confirm it
  names the module count it analysed.

### The recompute audit is part of the gate for engine changes

Any change under `internal/project/`, `internal/ledger/`, or `internal/oscal/`
must run the recompute-from-OSCAL test: export the fixture federation, recompute
every cell at every tier from the exported documents alone, and diff against the
served heat response. This is the acceptance claim in
[`docs/09-acceptance.md`](../09-acceptance.md); a change that cannot pass it is
a change to the claim, not to the code.

---

## Security pipeline requirements

Every check below must exist and must be able to fail. The question to ask of
each one:

> If the thing this step checks were completely absent, would this step still
> report success?

If yes, the step is decoration. A scanner that finds nothing because the code is
clean and a scanner that finds nothing because it never ran produce identical
artifacts unless you make them different — **assert the count, not just the exit
code.**

| Class | Tooling | Must prove |
|---|---|---|
| Secrets (gate) | TruffleHog, verified-only | A planted fixture under `tests/trufflehog-fixture/` is detected on every run. Without that job, a regressed scanner and a clean repo are indistinguishable |
| Secrets (evidence) | `secret-scan-hdf-emit.yml` | A **clean** scan still emits a one-control HDF recording the execution — scanner, version, commit, mode, zero findings. Thirteen estate repositories filed no secrets evidence at all because the obvious implementation skips the upload when nothing is found (`dev-sec-ops-baseline#21`) |
| SAST | `golangci-lint` + `gosec`; ESLint + `tsc --strict` for `web/` | Non-zero files analysed. A config file's presence is not analysis |
| Static analysis (service) | SonarCloud, project `risk-sentinel_sparc-horizon` | The project **resolves** before results are fetched, and the analysed line count is plausible for the tree. An unknown project returns an empty result that converts into a clean report. Keys are assigned at onboarding and do not follow a repository rename |
| Code scanning | CodeQL (`go`, `javascript-typescript`) | The detected language list is non-empty. Language detection reads the **default branch only** |
| Dependency | `govulncheck` (Go module graph) + Dependabot | The module count analysed is non-zero. A transitive advisory gets no Dependabot PR when nothing in `go.mod` names it directly — an empty dependency queue is not evidence of a clean graph (`sparc`'s `mail` advisory sat on `main` for five days that way) |
| SBOM | Syft → CycloneDX, via `container-build-sign`'s `sbom-source.yml` | The component count is greater than zero. An SBOM is inventory; it does not satisfy SCA |
| SCA | Grype (SBOM-driven) + Trivy `fs`, via `sca-scan.yml`, reconciled against `.security/sca-allowlist.yaml` | Both scanners converted to HDF. Every allow-list entry carries an expiry, and an expired entry suppresses nothing |
| Container | Trivy against the built image; `container-baseline.yml` for CRITICAL/HIGH dispositions | A CRITICAL/HIGH not in the baseline blocks the build. Scan the image that ships, not a dev variant |
| Signing | cosign, via `container-build-sign`'s `build-sign-publish.yml` | Signature and CycloneDX attestation verify for the published digest before anything consumes it |
| IaC | Not applicable in this repo — Horizon holds no Terraform | The `sparc-iac` issue covering Horizon's deployment carries the IaC scan |

### Conversion to HDF

- **Prefer ASFF over SARIF** when a scanner offers both. SARIF's three `level`
  values cannot carry a five-tier severity scale, so CRITICAL folds into HIGH at
  impact 0.7; ASFF's `Severity.Label` maps CRITICAL to 0.9. Where only SARIF is
  available, record `severity_fidelity` on the run and say so.
- **Verify the converter route before pinning it.** `hdf convert` has no source
  format for every tool, and a missing route is usually discovered as an empty
  output rather than an error. Confirm the route produces a populated HDF for a
  known-dirty input before trusting it on a clean one.
- **A clean scan converting to a document describing zero units renders as zero
  percent compliant, not as "clean".** Decide what a clean scan should say, and
  assert it says that.
- **Suppressions in the source format are frequently not carried across.**
  Reconcile after converting, and record who dismissed what and why.

### Evidence emission

Evidence goes to the canonical layout (`sparc-iac#537`):

```text
s3://<COMPLIANCE_S3_BUCKET>/risk-sentinel/<date|latest>/sparc-horizon/<source>/<file>
```

- `boundary` is **`risk-sentinel`**, set as a repository variable
  (`vars.EVIDENCE_BOUNDARY`) rather than inherited. Horizon projects posture
  *across* authorization boundaries and is not a component of the SPARC boundary,
  so filing under `sparc/` would assert a containment that is the opposite of
  what the tool does. Horizon is the first producer in the estate to use this
  value; the rest pivot onto it under `sparc-iac#715`.
- **No `|| 'sparc'` fallback in this repo.** The estate carries 24 of them and is
  removing them; a new repository has no reason to add a 25th. An unset boundary
  **fails the job and names the variable** — a fallback turns a misconfiguration
  into a valid write to the wrong boundary, which is worse than an error because
  the evidence exists, validates, and nobody is reading for it there.
- The emit role is `secrets.SPARC_HORIZON_EMIT_ARN`, scoped to
  `risk-sentinel/*/sparc-horizon/*`. It is provisioned in `sparc-iac` — file the
  issue, do not improvise a wider role.
- **Emit only on push-to-`main`, schedule, or dispatch. Never on
  `pull_request`** — a fork PR has no OIDC credentials and must not write the
  prod security bucket.
- **Verify what landed**, not that the upload step was green. A successful `cp`
  proves a request succeeded, not that the object holds what you think.
- **Stamp provenance** — commit, ref, run id, tool and version — at the time it
  is known, not reconstructed later.

### Workflow conventions

- Third-party actions are **SHA-pinned** with a comment naming the version;
  GitHub-owned `actions/*` stay tag-pinned, per estate convention.
- Reusable workflows from `container-build-sign` are pinned to a **SHA** with a
  dated comment saying what the bump was for.
- `sonarqube-hdf-emit.yml` is **self-contained and copied per repo** — a public
  repository cannot call a reusable workflow in a private or internal one, and
  that failure presents as a 0-second run with no jobs and no logs. Copy the
  file and change its CONFIGURATION block; do not convert it to a caller.
- **Lift a boolean, never a secret**, when a conditional needs to know whether a
  credential exists. `secrets` is not a valid context in a job-level `if:`, and
  using it there produces exactly that zero-job failure.
- Pin the runner image by **digest**, having verified its signature with cosign
  first. A tag can be re-cut onto a different image, and every run after that
  assesses with something else while the diff shows nothing.

---

## Branch protection

**Protection comes last, and a check name is added only after it has been
observed reporting at least once.**

A rule accepts any check name as a string. Requiring a check that never reports
blocks every pull request forever, and the block is invisible — it presents as
"expected, waiting for status". This bites hardest on a repository whose default
branch has no application code yet: CodeQL language detection and SonarCloud both
key off the default branch, so they report nothing until Go and TypeScript land
there, and requiring them first means the change that would fix it cannot merge.

Order that works:

1. Land the code on `main`.
2. Let each scanner run once and read **the exact name it reports** from the
   forge, not from the workflow file — the reported name is often the job name,
   or a name the external tool chooses.
3. Add those names as required checks.

Settings are copied from a known-good estate repository rather than invented, so
a reviewer can diff one against another:

```bash
gh api "repos/risk-sentinel/sparc-validate/rulesets" | jq -r '.[0].id'
gh api "repos/risk-sentinel/sparc-validate/rulesets/<id>"
```

- `strict_required_status_checks_policy: true`
- Reviews required before merge; `CODEOWNERS` covers the security-relevant paths
- Signed commits required
- Bypass mode **pull request only**, never **always** — "nobody commits directly
  to the default branch" is only delivered by the former. It still lets the owner
  merge their own PR, which matters where one person authors most changes. Some
  web UIs will not render the bypass-merge button under that mode; the CLI
  honours it, and that is a UI limitation rather than a reason to widen the
  bypass
- **A ruleset in evaluate mode reports its name happily while blocking nothing.**
  Assert on enforcement, not existence

Verify what is in effect, then prove it:

```bash
gh api "repos/risk-sentinel/sparc-horizon/rules/branches/main"
```

Then open a PR and confirm it is blocked for the reason you expect. A protection
rule nobody has tested is a claim.

---

## Go and TypeScript standards

- **`CGO_ENABLED=0`.** The storage layer is `modernc.org/sqlite` specifically so
  the binary stays static and the image stays distroless. A dependency that
  requires cgo is a design change, not a convenience.
- **Deterministic UUIDv5 over natural keys** (`uuid.NewSHA1`) for every object
  UUID, so reruns are idempotent and federated peers deduplicate without
  coordinating. A `uuid.New()` in a path that produces OSCAL is a bug.
- **The ledger is append-only and hash-chained.** No update or delete path, and
  no write from a what-if overlay. A test must prove the overlay writes nothing.
- **Schema changes must be forward-safe on a populated database.** The ledger is
  the system of record and `horizon rebuild` replays it, so a migration that
  cannot run against existing rows blocks a deployment. Guard every DDL step so
  it can be re-run after a partial failure, and never add a NOT NULL column
  without a default or a backfill in the same change.
- **Every handler on a node-scoped route checks the caller's role on the node**,
  not on the endpoint. A new route that omits the check is a security defect
  regardless of what it returns.
- **`web/` is `strict` TypeScript.** No `any` at an API boundary, no
  `@ts-ignore` without an approved suppression.
- **Accessibility is an acceptance criterion, not a polish pass** — WCAG 2.2 AA,
  keyboard reachable, blockers carry a glyph as well as colour, both themes pass
  contrast. axe runs in CI and its findings gate.

---

## References

- [`Implementation_plan.md`](Implementation_plan.md) — phased roadmap, security
  baseline, and the cross-repo issue table
- [`Developer_Collision_Avoidance_Plan.md`](Developer_Collision_Avoidance_Plan.md)
  — domain ownership and hot files
- [`../01-scope.md`](../01-scope.md) … [`../10-risks-decisions.md`](../10-risks-decisions.md)
  — the design of record
- [`../roadmap.md`](../roadmap.md) — product phase plan (P0–P8); its phase data
  is duplicated in `demo/planner.html` and `demo/full-plan.html`
- `risk-sentinel/dev-sec-ops-baseline` — `docs/sdlc/` defines what each SDLC
  stage must produce, and the `devsecops-coverage-*` / `artifact-*` controls are
  what Horizon's pipeline is measured against
- `risk-sentinel/container-build-sign` — `docs/dev/pipeline_design.md` and
  `docs/dev/sbom_sca_pipeline.md` are the contracts for the shared workflows
  Horizon calls
- `risk-sentinel/sparc-validate` — `docs/dev/issue_rules.md` carries the
  canonical suppression-approval text
