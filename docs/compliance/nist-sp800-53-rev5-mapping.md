# NIST SP 800-53 rev 5 — control mapping

Horizon's own controls. Read [`README.md`](README.md) first: the layer definitions and
the rule that a claim is worth what its evidence says are there, not repeated here.

**Status values.** `implemented` — the control operates and an artifact proves it.
`partial` — it operates over part of its scope. `planned` — designed, not built; the
design reference is the only thing behind it. `inherited` — another repository's
control, recorded so the boundary is visible.

**Scope note.** Horizon is at the design and prototype-plan stage. There is now a little
application source — the identifier grammar and the fixture generator (#38, #36) — and
exactly one application-layer row has moved off `planned` because of it. Everything else
is still design. That ratio is expected to invert through phases P1–P3, and an application
row that reaches `implemented` without a code location is a defect in this table.

---

## Implemented — repository and pipeline

| Control | Title | Implementation | Evidence | Location |
|---|---|---|---|---|
| IA-5(7) | No embedded unencrypted static authenticators | TruffleHog in verified-only mode gates every push and PR. A planted synthetic credential under `tests/trufflehog-fixture/` must still be detected, so a regressed scanner is distinguishable from a clean repository — without it the two produce identical artifacts | `risk-sentinel/<date>/sparc-horizon/trufflehog/trufflehog-hdf.json`, and the `fixture-detection` job | `.github/workflows/secret-scan.yml`, `tests/trufflehog-fixture/` |
| SA-11 | Developer testing and evaluation | Scanning runs in the pipeline and its results are normalised to HDF and filed as evidence, rather than read off a green check | The emitted HDF, read back from the bucket | `.github/workflows/secret-scan-hdf-emit.yml` |
| SA-11(2) | Threat modeling and vulnerability analyses | A threat model and security architecture review of the design of record, recorded as a dated attestation with a 180-day expiry and six early-staleness triggers. **Eleven findings as of r4**, none marked mitigated — the correct result for a repository with no application code — each carried as a named requirement on the phase that builds the affected component. **Currency is checkpointed, not continuous:** while a phase whose deliverables are the contracts the model reasons about is open, a trigger firing is logged in the model's interim staleness ledger and folded in at phase exit rather than re-attested per merge. A firing that contradicts a finding re-issues immediately regardless, and the triggers themselves are not narrowed (#31). **The P0-exit checkpoint was performed 2026-09-22 and produced r4** (#64): five firings folded in, TM-10 and TM-11 added, and the inherited-trust table extended. It also found the interim ledger had not been maintained — the firings were reconstructed from the git log — so the ledger is now a step-8 obligation in `issue_rules.md`. **This row does not claim continuous currency**; it claims a checkpointed review whose gaps are countable | [`threat-model.md`](threat-model.md) and its current attestation record, which carries a native `expires` so the review is countable rather than merely filed. **The ledger is the evidence for the interim**, and says which areas read as stale and for how long — so this row is not read as claiming continuous currency | `docs/compliance/threat-model.md`, `docs/compliance/attestations/` |
| SA-11(7) | Verify scope of testing and evaluation | Two canaries assert the assessors are still assessing. The secret-scanning fixture proves detection is live; `tests/actionlint-fixture/` plants an `SC2012` defect that workflow lint must report, because `actionlint` exits 0 when `shellcheck` is absent — it skips the integration rather than reporting it | Both canary jobs fail the build when the plant stops being found | `tests/trufflehog-fixture/`, `tests/actionlint-fixture/`, `.github/workflows/contracts.yml` |
| CM-3 | Configuration change control | Every change goes through an issue, a branch, a plan approved before code, the verification gate, and a five-section PR. `pr-checklist.yml` fails on an unchecked box outside the test plan | The PR record; the failing check when the template is not satisfied | `docs/dev/issue_rules.md`, `.github/PULL_REQUEST_TEMPLATE.md`, `.github/workflows/pr-checklist.yml` |
| CM-5 | Access restrictions for change | Branch protection ruleset on `main`, enforcement `active`: required reviews, required status checks with strict policy, and bypass limited to `pull_request` rather than `always`, so an administrator cannot push directly to the protected branch | Verified by attempting a direct push to `main` and reading the refusal — not by reading the ruleset back | Ruleset `main`; `.github/CODEOWNERS` |
| SA-10 | Developer configuration management | Third-party actions are pinned by commit SHA, not by tag. Tools are pinned by version and installed through checksum-verified paths: `actionlint` via the Go module proxy against `sum.golang.org`, `hdf-cli` by pinned release with checksum | The workflow sources; a bumped pin is a reviewed diff | `.github/workflows/` |
| SR-3 | Supply chain controls and processes | The same pinning, plus `npm install --ignore-scripts` with a direct binary call in place of `npx`, which would resolve a package from the registry on demand and run its install lifecycle scripts before the tool did anything. **A pin constrains which package is fetched, not what its install scripts do**, so both are required. Asserted rather than remembered: a check fails if any workflow invokes `npx` | The no-`npx` assertion in `contracts.yml`, which fails the build on a bare call, a `$(npx …)` substitution or one after a pipe (mutation-checked in #48) | `.github/workflows/contracts.yml`, `.github/workflows/sonarqube-hdf-emit.yml` |
| SR-11 | Component authenticity | Commits on `main` must be signed; the ruleset requires verified signatures | GitHub's signature verification on the branch | Ruleset `main` |
| AU-10 | Non-repudiation | Signed commits establish authorship of every change to the authorization-boundary record. Detached signatures over evidence are the application-layer half and are `planned` below | The signed history | Ruleset `main` |
| SI-2 | Flaw remediation | Dependabot opens PRs for the `github-actions` ecosystem. `gomod` and `npm` are added in S1, when those manifests exist — declaring them now would assert coverage of nothing | Dependabot PR history | `.github/dependabot.yml` |
| RA-5 | Vulnerability monitoring and scanning | **Partial.** Secret scanning is live and emits. Static analysis now runs on both sides: **findings for a pull request are fetched as OHDF before merge** (`sonar-pr-findings.yml`, #63) and the default branch's are emitted as evidence after. Historically it was wired but could not run — SonarCloud has never analysed this repository because there is no analysable source in it yet (#13) — and SCA, SBOM and `govulncheck` arrive with the first code, in S1 | The secret-scanning HDF; #13 records the gap rather than hiding it | `.github/workflows/`, `docs/dev/Implementation_plan.md` phase S1 |
| CA-7 | Continuous monitoring | Scans run on push to `main` and on a weekly schedule, so evidence does not go stale between merges, and each run files a dated object plus a `latest/` alias | `risk-sentinel/<date>/…` and `risk-sentinel/latest/…` | `.github/workflows/secret-scan-hdf-emit.yml`, `.github/workflows/sonarqube-hdf-emit.yml` |
| SR-4 | Provenance | Emitted evidence is stamped with the scanner and its version, the repository, the commit, the ref, the run id, the scan mode and the finding count, so an artifact in the bucket can be tied back to the run that produced it | The HDF control tags in any emitted object | `dev-sec-ops-baseline` `tools/scan_receipt.py`, via the reusable |
| SR-4 | Provenance (consumed artifacts) | A vendored third-party contract records where it came from — upstream repository, path, commit and SHA-256 — and a test recomputes the digest, so a copy edited to make a test pass is a build failure. Because that proves the copy is *unmodified* rather than *current*, a scheduled job compares it against upstream and files an issue when they part | `TestVendoredContractMatchesItsProvenance` recomputes the digest on every run; the scheduled job's summary states the two digests and the upstream commit it compared | `internal/keys/testdata/PROVENANCE.json`, `internal/keys/conformance_test.go`, `.github/workflows/vendored-contract-freshness.yml` |
| AC-6 | Least privilege | The emit role is scoped to this repository's prefix and deliberately not granted the `sparc/*` prefix the in-boundary producers hold, because Horizon sits above the SPARC authorization boundary. A write to the wrong boundary is refused rather than silently accepted | An attempted write outside the scoped prefix is denied | `.github/workflows/secret-scan-hdf-emit.yml`; role owned by `sparc-iac` |
| IA-5 | Authenticator management | CI authenticates to AWS by OIDC federation and receives short-lived credentials. No static access key exists to rotate, leak or scan for | The workflow's `id-token: write` permission and role assumption | `.github/workflows/` |
| SC-28 | Protection of information at rest | Evidence uploads send `x-amz-server-side-encryption: aws:kms` on the request rather than relying on the bucket default, because the bucket policy tests the request header and default encryption does not populate the condition key (#10) | `ServerSideEncryption: aws:kms` on the landed object, read back with `head-object` | `.github/workflows/secret-scan-hdf-emit.yml`, `.github/workflows/sonarqube-hdf-emit.yml` |

> **SR-3 was wider than its evidence until #48.** The row claimed `--ignore-scripts` with a direct
> binary call for this repository; that was true of `sonarqube-hdf-emit.yml` and false of
> `contracts.yml`, which ran five tools through `npx` inside two **required** status checks — 17
> registry resolutions per run of a gate that decides whether a pull request can merge. The claim
> is now true of every workflow, and a check asserts it rather than a reader remembering it.
> Recorded here rather than corrected silently: a mapping that quietly widens and narrows is worth
> less than one that says when it was wrong.

## Partial — application layer

| Control | Title | Status | Implementation | Evidence | Location |
|---|---|---|---|---|---|
| SI-10 | Information input validation | `partial` | Every field an identifier is derived from is validated and canonicalised before it is hashed, and **rejected rather than repaired** when it does not conform: a value carrying the unit separator, a control identifier outside its issuing vocabulary, a malformed period, a component that is not a UUID, an unresolved `source-uuid`. Rejection is the design choice — a repaired identifier still validates, still derives a UUID, and may name the wrong object. **Document-level validation starts here too** (#49): `internal/oscal` reads the OSCAL version and model a document declares and **rejects both when unrecognised** — an unsupported version is refused rather than decoded with the nearest types, and a root that is not an OSCAL model is refused rather than named. **What is still `planned`** is full schema validation on ingestion, which arrives with P1's adapters | Over seventy rejection cases across the `internal/canonical` and `internal/keys` tests — 76 at the time of writing, counted by hand — of which **11 are published** in `fixtures/key-vectors.v1.json`, so the Ruby and Python ports inherit the same refusals rather than re-deciding them | `internal/canonical/`, `internal/keys/`, `internal/oscal/`, `fixtures/key-vectors.v1.json` |

**Why this is one row and not three.** The same code is also the reason two federated peers
cannot mint different identifiers for one object, which is an integrity property rather than an
input-validation one. It is claimed once, here, under the control whose scope it actually sits
in; SI-7 stays `planned` until evidence resources are hashed and verified by running code rather
than by a fixture generator.

---

### One thing this table must keep saying out loud

Every emit path in the estate carries `continue-on-error: true` on the credential step.
A refused write therefore leaves the workflow **green**, and the GitHub API reports that
step's conclusion as `success` because `continue-on-error` masks it. Evidence for any row
above is the artifact in the bucket, or the `upload: … to s3://` line in the log — never
the colour of the check. An estate-wide outage ran for roughly forty minutes behind green
checks on 2026-09-19 (`sparc-iac#719`).

---

## Planned — application layer

No code exists for these yet. The design reference is the whole basis for the row, and
each becomes a step-9 obligation on the issue that implements it.

| Control | Title | Design | Reference |
|---|---|---|---|
| AC-2 | Account management | There are no Horizon accounts to manage. Identity comes from the agency IdP and roles come from `responsible-parties` in the OSCAL documents, so account state is a consequence of document edits | `docs/07-security.md`, `docs/03-data-model.md` |
| AC-3 | Access enforcement | Every API handler authorises against the **node** the request names, not the endpoint it reached. Roles bind to a node and inherit downward | `docs/07-security.md`, `docs/04-api.md` |
| AC-6 | Least privilege | Role bindings are per node; a role at one node confers nothing above it | `docs/07-security.md` |
| IA-2, IA-8 | Identification and authentication | OIDC with the agency IdP; party UUIDs map to subjects | `docs/07-security.md` |
| AU-2, AU-3 | Event logging, content of audit records | The ledger is append-only and records the decision, its inputs and its actor. Projections are materialised from it and rebuildable | `docs/05-projection-engine.md` |
| AU-9 | Protection of audit information | The ledger is hash-chained; exports carry the chain head, so a truncated or edited export is detectable by recomputation rather than by trust | `docs/05-projection-engine.md`, `docs/07-security.md` |
| AU-10 | Non-repudiation | Detached signatures over JCS-canonical (RFC 8785) JSON, with `signed-by` recorded in the namespace props | `docs/06-attestation-workflow.md` |
| SC-8 | Transmission confidentiality and integrity | mTLS to SPARC peers over the existing federation trust fabric; bundles verified before ingestion | `docs/07-security.md` |
| SC-12, SC-13 | Key establishment and management, cryptographic protection | Signing certificate and key referenced by `HORIZON_SIGNING_CERT`, held in the platform secret store — the store itself is inherited | `docs/08-build-deploy.md` |
| SI-7 | Software, firmware and information integrity | Evidence resources are hashed in back-matter; the chain from resource to observation to finding to risk to POA&M item is fixed and verifiable | `docs/06-attestation-workflow.md` |
| SI-10 | Information input validation | **Partial — see the row above.** What remains planned is validation of *documents* on ingestion; what exists is validation of the *fields* an identifier is derived from | `docs/03-data-model.md`, `schemas/` |
| CA-5 | Plan of action and milestones | POA&M items are first-class in the evidence chain, and an open milestone before the projection date is one of the three ways a control reads as down | `docs/05-projection-engine.md` |
| CA-7 | Continuous monitoring | The projection engine *is* the continuous-monitoring answer: state on date X, with expiry driven by the observation's native `expires` | `docs/05-projection-engine.md` |
| CM-2 | Baseline configuration | The container and env-var contract | `docs/08-build-deploy.md` |
| **What-if isolation** | *(no single 800-53 id)* | Overlays are copy-on-write, never persisted, and cannot write to the ledger or emit OSCAL or `saf attest` files. Stated as a security property, so the threat model (#12) has to show what **enforces** it rather than what intends it | `docs/06-attestation-workflow.md`, `docs/07-security.md` |

---

## Inherited — platform and pipeline

Recorded so the boundary is visible. Horizon claims none of these.

| Area | Owner | AWS component | 800-53 attribution |
|---|---|---|---|
| ECS Fargate service, task definition, task role | `sparc-iac` | Amazon EC2 Container Service | Pending `sparc#1103` |
| Evidence bucket, its policy and default encryption | `sparc-iac` | Amazon Simple Storage Service | Pending `sparc#1103` |
| KMS key for evidence at rest | `sparc-iac` | AWS Key Management Service | Pending `sparc#1103` |
| Emit role, OIDC trust policy | `sparc-iac` | AWS Identity and Access Management, AWS Security Token Service | Pending `sparc#1103` |
| Secret store for the signing key and OIDC client secret | `sparc-iac` | AWS Secrets Manager | Pending `sparc#1103` |
| Load balancer terminating TLS | `sparc-iac` | Elastic Load Balancing v2 | Pending `sparc#1103` |
| Service logs | `sparc-iac` | Amazon CloudWatch Logs | Pending `sparc#1103` |
| Image registry | `container-build-sign` | Amazon Elastic Container Registry | Pending `sparc#1103` |
| Image build, scan, SBOM, signature | `container-build-sign` | — | CM-14, SR-4, SR-11 at that repository |

**Why "pending" rather than a control id.** The AWS component definitions imported by
`oscal/cdefs/sparc-horizon.oscal.json` carry control implementations keyed to **AWS
Security Hub control ids**, not to 800-53. Writing that crosswalk here would be Horizon
authoring a mapping, which belongs to `sparc` — and is unnecessary, because SPARC already
ships the Security Hub to 800-53 rev 5 converter and ingests the AWS Labs definitions at
runtime.

The blocker is upstream and specific: `sparc#1103` reports those definitions importing as
a document with zero controls, so the converter has nothing to map. These rows therefore
name the component and cite the issue. An assessor reads that as an open dependency with
an owner, which is what it is — not as a claim, and not as a missing capability.

---

## Coverage

| Layer | implemented | partial | planned | inherited |
|---|---|---|---|---|
| Repository and pipeline | 16 | 1 | 0 | — |
| Application | 0 | 1 | 14 | — |
| Platform | — | — | — | 9 |

Counts are maintained by hand and checked at review. The application row moving off zero
is the measure that matters.

**A defect class worth naming.** SA-11(2) was missing from this table until #28, even though
the attestation's `reviewed-controls` selected `sa-11.2` from the day it was written — the
OSCAL record claimed a control the Markdown did not carry. Nothing reconciles control ids
between this file and `oscal/cdefs/` or `attestations/`, so the two can disagree silently in
either direction. Worth automating when OSCAL validation is wired into CI; `go-oscal` ships a
`validate` command, and the reconciliation is a short script on top of it.
