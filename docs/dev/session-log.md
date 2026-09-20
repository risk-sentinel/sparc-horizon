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
