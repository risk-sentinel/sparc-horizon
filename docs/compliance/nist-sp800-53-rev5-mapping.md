# NIST SP 800-53 rev 5 — control mapping

Horizon's own controls. Read [`README.md`](README.md) first: the layer definitions and
the rule that a claim is worth what its evidence says are there, not repeated here.

**Status values.** `implemented` — the control operates and an artifact proves it.
`partial` — it operates over part of its scope. `planned` — designed, not built; the
design reference is the only thing behind it. `inherited` — another repository's
control, recorded so the boundary is visible.

**Scope note.** Horizon is at the design and prototype-plan stage: there is no
application source yet. Every application-layer row is therefore `planned`, and the
`implemented` rows are all repository and pipeline controls. That ratio is expected to
invert through phases P1–P3, and an application row that reaches `implemented` without a
code location is a defect in this table.

---

## Implemented — repository and pipeline

| Control | Title | Implementation | Evidence | Location |
|---|---|---|---|---|
| IA-5(7) | No embedded unencrypted static authenticators | TruffleHog in verified-only mode gates every push and PR. A planted synthetic credential under `tests/trufflehog-fixture/` must still be detected, so a regressed scanner is distinguishable from a clean repository — without it the two produce identical artifacts | `risk-sentinel/<date>/sparc-horizon/trufflehog/trufflehog-hdf.json`, and the `fixture-detection` job | `.github/workflows/secret-scan.yml`, `tests/trufflehog-fixture/` |
| SA-11 | Developer testing and evaluation | Scanning runs in the pipeline and its results are normalised to HDF and filed as evidence, rather than read off a green check | The emitted HDF, read back from the bucket | `.github/workflows/secret-scan-hdf-emit.yml` |
| SA-11(7) | Verify scope of testing and evaluation | Two canaries assert the assessors are still assessing. The secret-scanning fixture proves detection is live; `tests/actionlint-fixture/` plants an `SC2012` defect that workflow lint must report, because `actionlint` exits 0 when `shellcheck` is absent — it skips the integration rather than reporting it | Both canary jobs fail the build when the plant stops being found | `tests/trufflehog-fixture/`, `tests/actionlint-fixture/`, `.github/workflows/contracts.yml` |
| CM-3 | Configuration change control | Every change goes through an issue, a branch, a plan approved before code, the verification gate, and a five-section PR. `pr-checklist.yml` fails on an unchecked box outside the test plan | The PR record; the failing check when the template is not satisfied | `docs/dev/issue_rules.md`, `.github/PULL_REQUEST_TEMPLATE.md`, `.github/workflows/pr-checklist.yml` |
| CM-5 | Access restrictions for change | Branch protection ruleset on `main`, enforcement `active`: required reviews, required status checks with strict policy, and bypass limited to `pull_request` rather than `always`, so an administrator cannot push directly to the protected branch | Verified by attempting a direct push to `main` and reading the refusal — not by reading the ruleset back | Ruleset `main`; `.github/CODEOWNERS` |
| SA-10 | Developer configuration management | Third-party actions are pinned by commit SHA, not by tag. Tools are pinned by version and installed through checksum-verified paths: `actionlint` via the Go module proxy against `sum.golang.org`, `hdf-cli` by pinned release with checksum | The workflow sources; a bumped pin is a reviewed diff | `.github/workflows/` |
| SR-3 | Supply chain controls and processes | The same pinning, plus `npm install --ignore-scripts` with a direct binary call in place of `npx`, which would resolve and execute a package — and its install lifecycle scripts — on demand | `.github/workflows/sonarqube-hdf-emit.yml` | `.github/workflows/sonarqube-hdf-emit.yml` |
| SR-11 | Component authenticity | Commits on `main` must be signed; the ruleset requires verified signatures | GitHub's signature verification on the branch | Ruleset `main` |
| AU-10 | Non-repudiation | Signed commits establish authorship of every change to the authorization-boundary record. Detached signatures over evidence are the application-layer half and are `planned` below | The signed history | Ruleset `main` |
| SI-2 | Flaw remediation | Dependabot opens PRs for the `github-actions` ecosystem. `gomod` and `npm` are added in S1, when those manifests exist — declaring them now would assert coverage of nothing | Dependabot PR history | `.github/dependabot.yml` |
| RA-5 | Vulnerability monitoring and scanning | **Partial.** Secret scanning is live and emits. Static analysis is wired but cannot run — SonarCloud has never analysed this repository because there is no analysable source in it yet (#13) — and SCA, SBOM and `govulncheck` arrive with the first code, in S1 | The secret-scanning HDF; #13 records the gap rather than hiding it | `.github/workflows/`, `docs/dev/Implementation_plan.md` phase S1 |
| CA-7 | Continuous monitoring | Scans run on push to `main` and on a weekly schedule, so evidence does not go stale between merges, and each run files a dated object plus a `latest/` alias | `risk-sentinel/<date>/…` and `risk-sentinel/latest/…` | `.github/workflows/secret-scan-hdf-emit.yml`, `.github/workflows/sonarqube-hdf-emit.yml` |
| SR-4 | Provenance | Emitted evidence is stamped with the scanner and its version, the repository, the commit, the ref, the run id, the scan mode and the finding count, so an artifact in the bucket can be tied back to the run that produced it | The HDF control tags in any emitted object | `dev-sec-ops-baseline` `tools/scan_receipt.py`, via the reusable |
| AC-6 | Least privilege | The emit role is scoped to this repository's prefix and deliberately not granted the `sparc/*` prefix the in-boundary producers hold, because Horizon sits above the SPARC authorization boundary. A write to the wrong boundary is refused rather than silently accepted | An attempted write outside the scoped prefix is denied | `.github/workflows/secret-scan-hdf-emit.yml`; role owned by `sparc-iac` |
| IA-5 | Authenticator management | CI authenticates to AWS by OIDC federation and receives short-lived credentials. No static access key exists to rotate, leak or scan for | The workflow's `id-token: write` permission and role assumption | `.github/workflows/` |
| SC-28 | Protection of information at rest | Evidence uploads send `x-amz-server-side-encryption: aws:kms` on the request rather than relying on the bucket default, because the bucket policy tests the request header and default encryption does not populate the condition key (#10) | `ServerSideEncryption: aws:kms` on the landed object, read back with `head-object` | `.github/workflows/secret-scan-hdf-emit.yml`, `.github/workflows/sonarqube-hdf-emit.yml` |

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
| SI-10 | Information input validation | Anything OSCAL cannot model is constrained by `schemas/sparc-namespace-props.v1.schema.json`; documents are validated on ingestion | `docs/03-data-model.md`, `schemas/` |
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
| Repository and pipeline | 15 | 1 | 0 | — |
| Application | 0 | 0 | 15 | — |
| Platform | — | — | — | 9 |

Counts are maintained by hand and checked at review. The application row moving off zero
is the measure that matters.
