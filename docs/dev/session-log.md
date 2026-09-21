# SPARC-Horizon Session Log

Continuity record for work in progress. Companion to
[`Implementation_plan.md`](Implementation_plan.md), which holds the roadmap and the
cross-repo table, and to [`issue_rules.md`](issue_rules.md), which holds the workflow.

**Last updated:** 2026-09-20 (**file created** under #33. Restores were costing a
reconstruction — six queries to re-derive branch state, merge status, phase position, and
upstream blockers — and the part that no query answers is work that stopped half-done before
it reached a commit.)

---

## What belongs here

GitHub is authoritative for everything it can express. This file holds only what it
structurally cannot.

| Goes in | Stays out |
|---|---|
| The exact stopping point of unpushed work: branch, what is uncommitted, the next command | Task or phase status — the epic's checkboxes are authoritative |
| Alternatives rejected that reached no PR body and no issue | Verification-gate measurements — the PR body's `## Verified by CI` holds these |
| Upstream blocker checks, with the date checked | Anything already in `Implementation_plan.md` or its cross-repo table |
| The recommended next slice, and why the others are blocked | Design rationale — that belongs in `docs/01`–`docs/10` |

**If GitHub can answer it, it does not go here.** Without that rule this file becomes a fourth
copy of the roadmap, and the repository already carries three duplication axes it would rather
not have.

The guardrail on account-specific identifiers applies to this file exactly as it applies to
commit, PR and issue text: no account numbers, ARNs, registry URIs, resource IDs or regions.

## How it is kept

Newest entry first, so a restore reads the top of the file and stops. One entry per session that
touched the repository, headed with the date, the issue numbers, and the branch.

**A rolling window, not an archive.** Roughly the last ten entries are kept; older ones are
trimmed. Git history already holds them and is the record of account. The file staying short
enough to read in full is the only property that makes it useful.

Updated in the same PR as the work it describes — `issue_rules.md` step 8. A continuity record
that updates out of band goes stale unnoticed, which is the failure this file exists to prevent.

---

## 2026-09-20 — #36 — `feature/36_fixture_federation`

**In flight:** nothing.

**`sparc#1155` checked 2026-09-20: still open.** The fixtures were generated anyway, under a
namespace derived from the placeholder URI rather than an invented constant —
`uuidv5(url-namespace, "https://risk-sentinel.org/ns/sparc")` — so any implementation in any
language reaches the same provisional value from the same published string. When the real
namespace lands it is one line, a regeneration, and a re-measurement of anything stated against
the old identifiers. **The P0 exit criterion is not closed by this PR**: the regeneration check
measures stability, not finality, and `sparc-validate` has not run against the tree.

**Generating the fixtures forced a resolution rule that the grammar did not state.** `source-uuid`
resolves to the UUID of the back-matter resource a `source` names — but back-matter resource
UUIDs are otherwise arbitrary, so a fresh one per citing document would give the same catalog a
different qualifier in every SSP that referenced it. That is the opposite of what the qualifier
is for. The convention adopted, and now written into `docs/03`: **a resource naming an external
catalog or profile carries that document's own UUID.** It is a clarification within v1, not a
field-list change, so no v2 and no grammar bump.

**The `Key` return type was a design choice, not an accident.** Every entry point returns the
canonical field list alongside the UUID. The ports in `sparc#1161` will disagree with this
implementation eventually; a bare UUID says only *that* they diverged, and the field list says
*where*. It is also what makes `fixtures/key-vectors.v1.json` — 26 vectors, 10 assertions, 11
rejections, 2 join cases — recomputable rather than a recording of what the generator happened
to emit.

**Rejected: `math/rand` with a documented seed.** It would have needed a `gosec` exclusion
(G404) against `.golangci.yml`'s stated empty-exclusion steady state, and "seed 11" does not mean
the same sequence in Go, Ruby and Python. A ten-line splitmix64 does. The same reasoning settled
the one conversion `gosec` flagged (G115): a bound check that provably cannot fire, rather than a
`nolint` comment — the check is the argument, written where a reader needs it.

**Rejected: one evidence artifact per observation.** 112 files whose only purpose is to be
hashed. One scan bundle and one attestation per boundary is what a real continuous-monitoring
run produces anyway, and the digests are of bytes the same run actually wrote — so the chain
resource → observation → finding → risk → POA&M item can be walked rather than trusted.

**Not done, and deliberately:** no Sonar exclusion for the generated fixture JSON. #42 is open
and owns that surface; adding one here would be a suppression without the record
`issue_rules.md` requires.

**Sonar had opinions, and one of them was right.** The quality gate failed on Security Rating,
from two `npx` findings in the new `fixture-props` job — and `npx` is the exact thing SR-3 in
`docs/compliance/nist-sp800-53-rev5-mapping.md` already claims this repository does not do.
The job now installs with `--ignore-scripts` and calls the binary directly, matching
`sonarqube-hdf-emit.yml`. **The other jobs in `contracts.yml` still use `npx` and predate this
PR**; that belongs to #42 rather than to a fixtures change touching required checks.

**Open, and needing the owner:** the third security finding is
`http://aws.amazon.com/ns/oscal` in `internal/fixtures/federation.go`, read as a cleartext
protocol. It is an OSCAL prop namespace — an identifier nothing dereferences — and rewriting
it to `https` produces a namespace AWS never issues, which breaks pass-through preservation.
Nothing was suppressed: the rationale is in the constant's doc comment and in PR #47, and the
disposition is the owner's to make in Sonar, where the platform records who decided.

**Next:** P0 has two tasks left — freeze API v0 with a mock server (unblocked, needs no
decision), and re-run the #26 round-trip probe against these fixtures before P1 builds
`internal/oscal`. The probe is now unblocked: SSP, profile and POA&M exist, which are exactly
the three models the spike could not exercise.

---

## 2026-09-20 — #37 — `feature/37_catalog_qualifier`

**In flight:** nothing.

**The grammar now partitions by catalog authority.** `source-uuid` — the UUID of the resolving
catalog or profile — is added to seven of the nine field lists, before `control-id`. Owner's
decision, provisional until a use case breaks it. Resolution is `control-implementation.source`
→ back-matter resource → **its UUID**, never the document-local `#fragment`, which would not
federate. Fallback when `source` is absent is the SSP's `import-profile`: component definitions
are component-scoped, while the profile holds the resolved control set and the ODP and org
statements.

**Amended v1 rather than bumping to v2**, against the document's own rule, on a verified fact:
nothing anywhere derives from v1's field lists, because the derivation needs the federation
namespace UUID and `sparc#1155` has not registered it. Recorded in the document as a one-time
exception, not a precedent.

**Corrected a claim I had made twice.** #37 item 3 and #42 both said the namespace schema
"rejects a spec-legal prop" because `ns` is required while OSCAL makes it optional. **That was
wrong**, and acting on it would have introduced a real weakness: the schema is a *selective*
validator applied only to props already in the sparc namespace, so relaxing `required: ns` would
let `{"name":"node-type","value":"boundary"}` validate as a sparc prop when an absent `ns` means
the NIST default. That is the spoofing the CI fixture guards against, in another form. The
constraint stays; the application rule is now stated in the schema description and in `docs/03`
so nobody applies it indiscriminately. 17/0 assertions unchanged.

**Canonicalisation is now scoped to a vocabulary.** `ACM.1` must not become `acm.1` — it would
still validate, still derive a UUID, and name nothing. `source-uuid` is what makes the vocabulary
decidable.

**Next:** #36's generator is unblocked. `sparc#1155` still blocks *freezing* the fixtures.

---

## 2026-09-20 — #13 follow-up — `fix/13_commit_time_without_checkout`

**In flight:** nothing.

**I broke the emit on `main` with #44 and this fixes it.** The wait step read the commit
timestamp with `git show -s`, and the `SonarQube -> HDF` job **has no checkout** — it talks to
SonarCloud and S3 and never needs the source. Every run failed with
`fatal: not a git repository`, exit 128, before reaching the wait logic at all.

**Why the PR could not catch it:** this workflow never runs on `pull_request` by design, so #44's
only real exercise was post-merge. That was stated in its own post-merge section, and the check
found the defect immediately — the process worked, the change was wrong.

**What I should have checked:** the job's own steps. The comment directly above them says the
project key is derived from `GITHUB_REPOSITORY` "rather than `github.event.repository.name`,
which is not populated on `schedule` runs" — a job that careful about context availability was
signalling a minimal footprint, and I assumed a checkout into it anyway.

**Fix:** read the commit timestamp from the GitHub API rather than git. Not from
`github.event.head_commit.timestamp` either, which is unpopulated on `schedule` runs — the same
trap the existing comment warns about. Verified by dispatch:
`committed 2026-09-20T14:34:42Z` resolves and the poll loop runs.

---

## 2026-09-20 — #13 — `fix/13_sonar_emit_race`

**In flight:** nothing. The emit workflow waits for the analysis of the current commit before
fetching.

**#13 was a race, not a shape mismatch.** That question had been open all day and is now settled
by experiment: the emit fails on push and **succeeds unchanged when re-run minutes later**. On
the #43 merge the analysis ran 14:19:54–14:20:10 while the job fetched at 14:20:00 and failed at
14:20:09 — one second early. `hdf-cli` was never wrong about the API shape; it was rejecting an
empty response.

**The evidence path is proven end to end.** A dispatched re-run emitted OHDF with **12 rule
types and 103 failed results** — 95 in `demo/`, 8 in `.github/workflows/` — schema-valid, with
commit and run provenance stamped on, to both the dated and `latest` bucket paths.

**Two flaws found while fixing it, both in my own work:**

- Waiting for the Sonar queue to drain is **not sufficient**. An empty queue can mean *not queued
  yet* rather than *finished*, if the push webhook has not been processed — so the first version
  of the fix would have read the previous commit's analysis and failed on a false negative. The
  condition is "does an analysis of THIS commit exist", polled, not "is the queue empty".
- A stale analysis is worse than no analysis. The Label step stamps the **current** commit onto
  whatever was fetched, so an older analysis produces evidence that is well-formed, schema-valid
  and attributed to code it was not derived from.

**The old guard could not have caught either.** `Resolve and verify` calls `api/components/show`,
which proves the project **exists** — a project that has never been analysed passes it. Confirmed
by dispatching against this branch: the new assertion fired with exactly that case.

**shellcheck earned itself immediately.** It caught a stray quote that a Python `.rstrip()` had
eaten out of the new step — a genuine syntax error, on the first bundle where shellcheck was
available locally.

**Next:** #37's qualifier shape still gates #36. The API v0 freeze remains unblocked.

---

## 2026-09-20 — #42 — `feature/42_sonar_exclusions`

**In flight:** nothing. `sonar-project.properties` added; #42 stays open for the analysis-method
decision and the post-limit re-test.

**Root cause of the Sonar failures, found from the console rather than the repo:** the
organization exceeds its SonarCloud line-of-code limit, so **every** internal project's analysis
fails regardless of size. Horizon is ~1,800 tracked lines, about **0.1%** of the counted total.
Filed upstream as `sparc-validate#410`: 89.5% of that repository's tracked lines are
`benchmarks/` XCCDF data (714K) and `.oscal-cache/` NIST catalogs (255K), neither of which is
source.

**Correction worth not repeating:** a first pass at those numbers counted the **working tree**
and blamed `overlays/` (215M) and `profiles/` (116M). Both are almost entirely **untracked**, so
they never reach a CI checkout. Count with `git ls-files` when the question is what CI analyses.

**Why this landed before analysis works:** the exclusions protect against findings nothing is
currently producing, which makes them look premature. They are not. The moment the org limit
clears, the first successful run reports the planted canaries in `tests/` and the CI-enforced
duplication in `demo/` as real findings, and someone resolves one.

**Deliberately not set:** `sonar.go.coverage.reportPaths`. Correct setting, but Automatic
Analysis does not run in the pipeline and can never read `coverage.out`. Setting it would imply
coverage is reported when it is not. Recorded as a comment in the file so the absence reads as a
decision rather than an oversight.

**Next:** #37's qualifier shape still gates #36. The API v0 freeze remains unblocked.

---

## 2026-09-20 — #40 — `feature/40_pin_shellcheck`

**In flight:** nothing. `contracts.yml` pins shellcheck; the collision plan's pin table replaces
the narrower §2.4b.

**Environment change worth not rediscovering: `shellcheck` is now installed on this machine.**
Every bundle before this one recorded workflow lint as "schema and expressions only — CI measures
the shellcheck half". **That is no longer true.** The canary fires locally and all workflows lint
with the integration active.

Two versions, because **v0.9.0 has no `darwin.aarch64` build**: 0.10.0 native at
`~/.local/bin/shellcheck`, and 0.9.0 (`darwin.x86_64`, Rosetta) at `~/.local/bin/shellcheck-0.9.0`
for parity with CI. Installed from release tarballs, not `brew`.

**Found while installing it:** CI's shellcheck was **unpinned** — whatever `ubuntu-latest`
shipped. Neither existing guard caught that: the presence check passes because some shellcheck is
there, and the canary passes because `SC2012` is stable. The lint's meaning could change between
two runs of identical code with every signal green. The job's own error text had anticipated it
("add a pinned shellcheck install to this job") without acting on it.

**Decided:** pin to **0.10.0** rather than to 0.9.0, the version CI happened to be running.
Pinning to an older release we would immediately want to leave is ceremony. Verified first that
the bump is behaviour-neutral here — both versions lint all six workflows at exit 0 and both fire
the canary — so this is a pin, not a silent upgrade.

**Next:** #37's qualifier shape is still with the owner, and gates #36. Unblocked: freezing API
v0 and the mock server, and #37 item 3.

---

## 2026-09-20 — #38 — `feature/38_go_toolchain_ci`

**In flight:** nothing. The Go toolchain, `ci.yml`, and `internal/canonical` are complete and
the gate passes locally.

**Found the hard way, and now encoded in the workflow:** `gofmt -l` **exits 0 while listing
unformatted files**. A `gofmt -l . && echo clean` reported success over a file gofmt was
actively rejecting — the exact failure the fixture and canary guards exist to prevent, committed
by the check itself. `ci.yml` asserts the list is empty rather than trusting the exit status, and
says why in a comment.

**Tooling installed outside the repo:** `golangci-lint` v2.6.2 and `actionlint` v1.7.12 via
`go install` — not `brew`, which is a hard guardrail. `golangci-lint` v2 uses a different config
schema from v1: `version: "2"`, `linters.default`, `linters.settings`, `formatters`, and
`excludes` must be `[]` rather than an empty key. `config verify` catches it.

**Still no shellcheck on this machine**, so actionlint's shellcheck integration is inactive
locally and CI measures that half. Unchanged from previous bundles.

**Decided:** the seed package is `internal/canonical` rather than a placeholder. CI asserting a
coverage floor over zero packages would be theatre, and the module needs a real package for the
assertion to mean anything. It carries only what #37 cannot change — separator rejection, period
and UUID forms, NFC — and deliberately **excludes control-id canonicalisation**, because that
rule is valid only within a vocabulary and #37 has not scoped it yet.

**Next:** #37's qualifier shape is with the owner. #36's derivation and generator wait on it.
Unblocked meanwhile: freezing API v0 and the mock server, and #37's item 3 (the schema
accepting an absent `ns`).

---

## 2026-09-20 — #31 — `feature/31_attestation_cadence`

**In flight:** nothing. Working tree clean at the point this entry was written; the bundle is
the cadence rule, r3, and the step 8 updates.

**Decided:** the cadence is **checkpoint at phase exit** while a contract-defining phase is
open — triggers unchanged, interim firings logged, a firing that contradicts a finding
re-issues at once. Options 2 and 3 from #31 were rejected: narrowing the triggers swaps a
bright line for a judgement call made by whoever would rather not re-attest, and re-issuing
per change erodes review quality within a week.

**Found while doing it, and worth not rediscovering:** resolving #31 *forced* an r3 whichever
option was chosen. The cadence rule is written in the Freshness section of the attested
document, and the current record's back-matter binds to that file's SHA-256, so recording any
cadence changes the hash and requires a new record. The rule that reduces re-attestation could
not be adopted without one more re-attestation. The expiry deliberately does **not** reset:
2027-03-18 is 180 days from the 2026-09-19 substantive review, and resetting on each revision
would make a bounded interval perpetual.

Also recovered by brute force, because it was written down nowhere: attestation UUIDs derive as
`uuidv5(uuidv5(URL, "https://risk-sentinel.org/ns/sparc"), "<kind>:threat-model-<review-date>")`,
with `resource:superseded-attestation-threat-model-<date>` for the supersession link. The
subject and process-of-record resource UUIDs are stable across revisions and were reused
verbatim; their keys are still unknown and were not needed.

**Upstream checked (2026-09-20):** unchanged from the entry below — `sparc#1155` and
`sparc#1161` both open.

**Next:** #31 stays **open** until the P0-exit checkpoint folds in anything further. The next
P0 slice is the fixture federation generator, which gates the #26 round-trip probe.

---

## 2026-09-20 — #33 — `feature/33_session_log`

**In flight:** this file, plus the step 8 bullet, the `CLAUDE.md` pointer, and the
`Developer_Collision_Avoidance_Plan.md` 2.4 extension. Nothing pushed yet.

**Decided:** a local [beads](https://github.com/steveyegge/beads) board was considered as a
second tracker and rejected. It has no GitHub Issues integration — its `git+https://` remote
stores Dolt's own history and never reads or writes issues — so hybridising means hand-maintained
dual entry. Its state is local, so CI cannot gate the drift the way `Duplicated copies agree`
gates the three existing axes. Six bindings keep issues on GitHub regardless: PR auto-close,
branch protection, cross-repo filing into sibling repositories, the org project board, the phase
milestones, and assessor visibility. Recorded here rather than in #33 only because the reasoning
is as useful as the conclusion. A local scratch layer below the issue grain remains available as
a separate decision, and would never mirror closed GitHub state.

**Next:** #31 (attestation cadence) or the fixture federation generator. See the entry below.

---

## 2026-09-19 — S0 close-out and the first three P0 slices — reconstructed

**This entry is reconstructed from git history, merged PRs and issue state, not written
contemporaneously.** Every merge below landed on 2026-09-19, so the session boundaries within
that day are not recoverable and have not been invented. It exists to give the rolling window a
floor, and it is the only entry in the file that was not written by the session it describes.

**Landed:** #5, #3, #7, #9 (S0 pipeline work), #14 (S0 close-out), #27 (#26 — OSCAL type layer),
#29 (#28 — threat model r2), #32 (#30 — UUIDv5 key grammar). `main` at `97685ca`.

**Position:** S0 complete. P0 three tasks done — the namespace schema landed early as S0-7, the
key grammar is normative, and the OSCAL type layer is decided. Three remain, tracked on #15.

**Decided, and not otherwise written down:** the Go reference implementation of the key grammar
was deferred to P1 rather than written in P0, because the first `.go` file in the repository
pulls S1's `go.mod`, CI and coverage obligations with it, and those should arrive with S1 rather
than ahead of it. Spike code ran in a scratch module outside the repository for the same reason.

**Open decision:** #31 — the threat model attestation tripped its own staleness triggers twice in
one day. **r2 is stale and is not to be read as current.** Recommendation on the issue is to
checkpoint at P0 exit.

**Upstream checked (2026-09-20):** `sparc#1155` open — the federation namespace UUID is
unregistered, so fixture UUIDs cannot be frozen, though the generator is buildable now.
`sparc#1161` open — Ruby and Python ports of the key grammar plus the shared test vectors.

**Next:** #31 closes the only currently-stale artifact and is cheap. The fixture federation
generator is the critical path to P1 — it gates the #26 round-trip probe, which exercised only 3
of 7 OSCAL models and never touched SSP, profile or POA&M, and SSP is the model Horizon leans on
hardest. Freezing the API and standing up the mock server is unblocked but off that path.
