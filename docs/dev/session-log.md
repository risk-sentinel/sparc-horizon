# SPARC-Horizon Session Log

Continuity record for work in progress. Companion to
[`Implementation_plan.md`](Implementation_plan.md), which holds the roadmap and the
cross-repo table, and to [`issue_rules.md`](issue_rules.md), which holds the workflow.

**Last updated:** 2026-09-24 (#57). (**File created** 2026-09-20 under #33. Restores were costing a
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

## 2026-09-24 — #57 — `fix/57_verify_pinned_shas`

**In flight:** nothing. Second of the stack, branched from #79. Merge order: #79, #57, #42.

**All five pins are currently correct** — measured before writing anything, by resolving each SHA
through the GitHub API. So the check passes on today's state, which means it proves nothing until
it is shown to fail. Five mutations do that, and the first is #56's actual defect: `setup-go`
labelled `v6.0.0` while the SHA is `v7.0.0`. Also caught: a comment removed entirely, a major alias
(`v7`) in place of a specific version, a SHA no tag points at, and the matcher ceasing to match.

**The count floor is the one that matters.** The obvious implementation greps, loops and reports
success over an empty list the first time a file is renamed or the `uses:` spelling changes — the
same failure `ci.yml`'s `MIN_TESTED_PACKAGES` exists to defeat. It asserts at least five pins and
says so when it finds fewer.

**Scheduled, not required, and that is a trade rather than a preference.** Resolving a SHA means
calling `api.github.com` about somebody else's repository. Requiring it would make every pull
request here depend on an external service, which is exactly what #68 refused for
`vendored-contract-freshness.yml`. The cost is that drift is caught shortly after a merge rather
than before it. That is the right way round: a stale comment is a legibility defect, not a live
vulnerability, and blocking every merge on GitHub's API availability to catch it an hour sooner is
the worse failure. It runs on merges touching workflows, weekly, and on demand — twenty minutes
after the vendored-contract check so two scheduled jobs are not competing for one rate limit.

**An API error must not read as a pass.** A failed `gh api` call exits the job rather than being
treated as "no tags found", which would have reported a mislabelled pin as an unfindable one.

**SA-10 now cites the check.** The row claimed actions are pinned by SHA "so a bumped pin is a
reviewed diff", and that rests entirely on the comment being true — which nothing verified. Same
shape as SR-3 in #48, caught before it became an overclaim rather than after.

**Next in the stack:** #42 — Sonar configuration, which #63 added a finding to: `_test.go` files
are held to a reduced rule set.

---

## 2026-09-24 — #79 — `feature/79_mockserver_authz`

**In flight:** nothing. First of a stack — #57 branches from here and #42 from that, so each branch
carries its predecessors and the last PR holds the whole chain. Merge in order: #79, #57, #42.

**The mock decided refusals from a file.** `newServer` flattened each persona's committed
`tree.json` into a visibility map, so if the generator and `internal/authz` ever disagreed, the mock
sided with the generator — and `GET /v1/tree` returned bytes, which cannot demonstrate the one
thing the endpoint exists to show. It now builds the tree from `fixtures/oscal/ssp-*.json` and
answers both from `internal/authz`, the packages the service will use.

**#76 left this undone deliberately, and collecting on that debt was the point.** While the mock
walked goldens and `internal/authz` computed, the two could disagree observably. That check does
not disappear — `TestSubtreeMatchesTheFrozenPersonaGoldens` still pins the computed subtree to the
frozen goldens on every run. What goes away is a second, weaker implementation of the same rule.

**Two mutations exposed real gaps in what the tests proved, and both are now closed.**

*One.* Deleting the visibility check in `s.node()` failed **nothing**. The reason is precise rather
than alarming: each persona has its own `api/<persona>/heat/` directory, so a foreign node 404s
because the file is absent, not because authorization refused. The endpoint where authorization is
genuinely load-bearing is `chain`, whose goldens are **shared** across personas — and neutralising
*that* check does fail, with "a chain outside the caller's tree returned 200, want 404". Worth
knowing which of the two is actually protecting the response.

*Two.* Making `/v1/tree` serve the golden again also failed nothing, because the computed tree and
the golden agree — which is the whole point, so equality cannot distinguish them. `TestTreeIsComputedNotRead`
closes it by serving a filesystem with `api/<persona>/tree.json` **hidden**: if the handler still
answers correctly, it cannot have read it. That mutation now fails with "returned 404 with the
golden hidden — it is being read, not computed".

**The existing six tests needed two edits, both mechanical** — a `s.visible[id][node]` premise check
and a chain selector became `s.authz.Visible(party, node)`. Same assertions, asked of the
implementation instead of a map. The issue said an edit here would be a signal worth reading; it
read as "these tests assert the contract, not the mechanism", which is what they should do.

**The server now refuses to start without the OSCAL fixtures.** Starting anyway would make every
node invisible and every response a 404 — indistinguishable from a healthy mock that simply denies
you, which is the worst available failure.

**Next in the stack:** #57 — nothing checks that a pinned SHA is the version its comment claims.

---

## 2026-09-24 — #48 — `fix/48_npx_in_required_checks`

**In flight:** nothing. Chosen over more P1 code because P1's unblocked work is thin — the client
waits on `sparc#1181` and the mapping documents on `sparc#1154` — and because this one was not
merely a hotspot.

**SR-3 claimed something that was not true of this repository.** The row in
`nist-sp800-53-rev5-mapping.md` said Horizon installs with `--ignore-scripts` and calls binaries
directly "in place of `npx`". That was true of `sonarqube-hdf-emit.yml` and **false of
`contracts.yml`**, which ran five tools through `npx` inside two *required* status checks — 17
registry resolutions per run of a gate that decides whether a pull request can merge. A pin
constrains which package is fetched, not what its install lifecycle scripts do, so the pins that
were there were necessary and not sufficient. The overclaim is recorded in the mapping rather than
quietly corrected.

**The fix was already written, in the same file.** `fixture-props` carried the pattern with its
reasoning; the other five call sites had simply never been converted. One install per job replaces
one resolution per call.

**Verified by running it, not by reading it.** Installed both tool sets locally with
`--ignore-scripts`, confirmed the binaries `@redocly/cli` actually ships (`redocly`, `openapi`,
`js-yaml`) rather than assuming the names, and re-ran the namespace-schema job's assertions
through the direct binary: **17 passed, 0 failed**, the same numbers CI asserts. `js-yaml` reports
the same 11 paths.

**The guard is the part worth keeping.** The fix is one line from being undone by whoever adds the
next tool, and it would be undone inside a required check — so a step now fails the build if any
workflow invokes `npx`. Matching prose would have been worse than nothing: the step's own name and
error message mention npx, and a guard that trips on its explanation is one somebody disables. It
matches only where a command can begin, and is mutation-checked four ways — a bare call, a
`$(npx …)` substitution (the shape line 38 actually had), one after a pipe, all caught; a
commented-out one correctly ignored.

**I deleted a required check and every other check went green.** Rewriting the guard step used
`s[:start] + new`, which truncated everything after it — taking the `duplication` job
("Duplicated copies agree") with it. `gh pr checks` then reported **10 of 10 passing** and said
nothing about a required context that had stopped existing. Caught by noticing the count was 10
where previous PRs had 11, not by any check.

**Branch protection is what would actually have stopped it.** "Duplicated copies agree" is one of
the seven required contexts, and a required check that never reports blocks the merge, so the PR
could not have landed. The safety net held. What did *not* hold is the signal a reviewer looks at:
an all-green check list is **not** evidence that every required check ran, and nothing local says
otherwise.

No new guard was added for it. The protection ruleset already covers it, and an assertion pinning
the job set would fail on every legitimate removal — a check that cries wolf gets deleted. The
lesson is about the edit, not the pipeline: **a slice-and-replace on a config file needs the tail
put back, and the cheapest proof is comparing the parsed job set against `main`**, which is what
found it and what confirmed the restore.

**Next:** P1's unblocked remainder is thin. `cmd/mockserver` could be rewired onto
`internal/authz` — deliberately left out of #76 so the mock and the implementation can disagree
once, observably — or #57 (nothing verifies a pinned SHA matches its comment) and #42 (Sonar
configuration). Both upstream asks were checked today and are unmoved: `sparc#1154` since
2026-09-22, `sparc#1181` since 2026-09-23.

---

## 2026-09-24 — #76 — `feature/76_node_authz`

**In flight:** nothing. Second P1 slice; `internal/authz` decides visibility over #74's tree.

**Building the consumer found two defects in #74, which is the whole argument for building it.**
Both were invisible to #74's own tests and both were caught by the persona goldens frozen in #61 —
written before either package existed.

*One.* **The system tier was keyed on the wrong OSCAL element.** #74 built system nodes from
`inventory-items`, justified by the count: 20 items against 20 documented systems. The count was
right and the identity was wrong — the frozen goldens identify a system by its **component** uuid
and title. The rule that actually holds is better than either reading: **a system is a component
that has an inventory record.** That uses both halves of what `docs/03-data-model.md` names, gives
component identity, gives exactly 20, and excludes the inherited AWS component for the right
reason — it has no inventory record because it is not Horizon's system to inventory, which is what
makes it not a node, rather than a special case.

*Two.* **Sorting the system tier reordered it away from the contract.** `sortTree` sorted every
tier by name. Organizations and boundaries are assembled across documents and need an ordering, or
they follow the filesystem. Systems come from **one** document and already carry its declaration
order, which is both stable and meaningful. Sorting them cost nothing structurally and made the
tree disagree with the API it exists to serve.

**`Node.roles` in a response is the CALLER'S roles, not every role bound there.** The goldens
settle it — `so-ods-portal`'s boundary carries only `system-owner`, although an ISSO is bound there
too. Right twice over: the HUD asks "what may I do here", and listing every holder would disclose
an organization's staffing to anyone who can see the node. `internal/tree` keeps the full set
because the documents declare it and the recompute audit needs it; they are different questions.

**#74 threw away information it had.** `Node` is the frozen contract shape,
`additionalProperties: false`, so it cannot carry a party uuid — and the binding's party was simply
dropped after the role was attached. It is unrecoverable downstream, because a party bound at an
organization need not appear in any document beneath it. `tree.Result` now carries `Bindings`
beside the tree.

**Three things the design does not specify, recorded rather than assumed** (`docs/10-risks-decisions.md`):
how an OIDC subject maps to a party uuid — asserted by `docs/07-security.md`, specified nowhere,
and not expressible against fixtures whose parties carry no `external-ids`, props or emails;
sibling order in `children`, which the contract leaves open while the P0 goldens encode
`internal/fixtures`' own table order, determined by no document; and what a party bound at two
disjoint nodes should be shown, when `/v1/tree` returns exactly one `Node`.

**Mutation-checked.** Making visibility by mention rather than by position turns all three persona
goldens red plus the inheritance test; making an absent node answer differently from an
unauthorized one turns the refusal test red on all three of its assertions. The second is the
disclosure TM-1 is about, and it is the reason there is no `Exists` method to call.

**Next:** P1's remaining unblocked task is control-id normalisation against SPARC mapping documents
— the mappings are `sparc#1154` part 2, so only the local half is reachable. TM-1's other two
requirements need a signed bundle and the ledger (P2). `internal/sparc` stays blocked on
`sparc#1181`.

---

## 2026-09-24 — #74 — `feature/74_node_tree`

**In flight:** nothing. First P1 slice; `internal/tree` builds and tests against the OSCAL
fixtures, so nothing here waits on SPARC.

**The frozen contract decided the slice, not me.** `Node` in `api/openapi.yaml` makes `roles`
required, so a tree built without them is invalid by construction and splitting "tree" from
"roles" would have produced a package whose output no handler could serve. Tasks 3, 4 (partly)
and 5 of #16 therefore land together, and both of that epic's exit criteria are met.

**Role placement comes from the tier table in `docs/03-data-model.md`, and it had to.** Every role
holder in the fixtures is `member-of` the **organization** — AO, system owner and ISSO alike — so
membership cannot distinguish them. The rule is by role: AO binds at the organization, SO and ISO
at the boundary, and the system tier reads component `responsible-roles` through the inventory
item's `implemented-components`, because an inventory item carries no responsible-parties of its
own. Measured before writing any of it.

**`inventory-items` are the system tier, not `components`.** `CLAUDE.md` says
"`components`/`inventory-items`", which is ambiguous, and the counts settle it: 20 inventory items
against the federation's documented 20 systems, while the 27 components include the inherited AWS
platform. Building from components would have produced a 27-node tier that passes every structural
test and describes something that is not the federation.

**Bindings, not effective roles.** The contract calls the field "role bindings" and
`docs/07-security.md` says roles inherit downward — so inheritance is an authorization question
evaluated against a request, not something baked into the tree. Pre-expanding it would make the
tree lossy about where a role was *declared*, which is what the recompute audit asks. Owner
confirmed this reading before implementation.

**A divergence from the architecture doc, resolved in the doc rather than in silence.**
`docs/02-architecture.md` assigned "responsible-parties to node-scoped roles" to `internal/authz`,
and the contract forces bindings into `tree`. The line now reads: `tree` carries what each node
declares, `authz` turns it into a decision. `CLAUDE.md` is explicit that code contradicting a
design doc is a bug in one of them, never an accepted divergence.

**Five mutations, all caught.** A no-op `note()` turns the five finding tests red; binding AO at
the boundary names the exact wrong placement; building systems from components trips the count
test at 27 against 20; leaving `Roles` nil fails the frozen schema. That last one is the value of
compiling `Node` out of `api/openapi.yaml` in the test rather than restating its enums in Go —
restated enums would agree with whatever the builder believed.

**Two lint findings fixed rather than suppressed.** `.golangci.yml` keeps an empty gosec exclusion
set on purpose. G304 on `os.ReadFile` over a glob was answered by reading through an `fs.FS` rooted
at the fixture directory, so nothing outside it is reachable by construction — the property the
rule is actually about — and revive's unused parameter by deleting it.

**Left alone deliberately:** no NIST mapping row moves off `planned`. This slice *derives* the
bindings; it enforces nothing, and `docs/compliance/README.md` is explicit that a row with no
evidence stays planned. AC-2, AC-3 and AC-6 move when `internal/authz` can refuse something.

**Noticed, not fixed:** the status snapshot's "NIST control coverage: **0 documented**" row is
stale — `docs/compliance/` has existed since S0. Out of scope here; flagged rather than quietly
corrected or quietly ignored.

**Next:** `internal/authz` — OIDC subjects, downward inheritance and the node-scoped 404 over this
tree, which is TM-1's third requirement and needs nothing from SPARC either. `internal/sparc`
stays blocked on `sparc#1181`.

---

## 2026-09-24 — #72 — `fix/72_sonar_closed_findings`

**In flight:** nothing. This branches from `main`, not from #70's branch, so only one workflow PR
is open at a time — #71 is open and green but touches no workflow. **On merge, this entry and
#70's will both want the top of this file**; #70's is dated 2026-09-23 and belongs below this one.

**The PR-findings workflow reported a finding that had already been fixed.** PR #71 carried one
`go:S3776`, commit `3b61177` fixed it, and the next run still said `Reported 1 result(s)`. The
cause is that `issues/search` returns CLOSED issues unless told not to, and the count query never
said `resolved=false`. Measured on PR #71 after the fix landed: the query as written returned
`total=1` with `status: "CLOSED"`; with `resolved=false` it returned `0`.

**`hdf fetch` does not filter either**, so fixing only the count would still render stale findings
whenever something else was open. OHDF distinguishes them, and this was measured across two real
artifacts rather than assumed — PR #67's planted finding (Sonar OPEN) is `status: "failed"`, and
the same rule on #71 after the fix is `status: "passed"`. So the summary now counts and renders
`failed` only, and says out loud how many resolved results it left out.

**Found by using the thing.** #63 shipped with a scratch PR proving the reporting path on a planted
finding, which exercised the mechanism but not this: a finding going from open to closed *inside*
one pull request. The plant was never fixed, so the state that breaks it never arose. A proof built
from a fixture covers the path that fixture walks, and the first real use walked a different one.

**Why this matters more than the size of the diff.** The workflow exists so the report can be
believed without opening the dashboard. One listing of already-fixed findings teaches the reader to
check the dashboard anyway, which is the habit #63 removed. It is also #13's failure — stale data
attributed to current code — arriving from the opposite direction.

**Next:** **P1** (#16), `internal/tree` over the OSCAL fixtures. The client task waits on
`sparc#1181`; the tree builder and role mapping need nothing from SPARC.

---

## 2026-09-23 — #70 — `feature/70_correct_sparc_fixtures`

**In flight:** nothing.

**P1 was about to be built on a contract that does not exist.** `fixtures/sparc/` is P1's test
oracle, and `fixtures/README.md` already warned it was written from SPARC's API *documentation*.
Re-reading the documentation proves nothing, because that is where the fixtures came from — so
this was measured against `risk-sentinel/sparc` `origin/main` (`bb82c75f`), reading controllers
and serializers. Four things were wrong, and every one of them would have passed Horizon's tests:
the envelope (`{data, meta}`, not `{collection, endpoint, note, count, data}`), **no pagination at
all**, the route name (`authorization_boundary_memberships`), and an invented `oscal_party_uuid`.

**The invented field is the one that mattered.** `Organization#uuid` is `gen_random_uuid()` — the
model calls it "the stable audit identifier" — and the only `party_uuid` in SPARC's schema is on
`ssp_leveraged_authorizations`. The fixtures had encoded the assumption that SPARC hands Horizon
the join key relationally. It does not, and it should not: `ssp_documents[].uuid` is the single
OSCAL identifier in the whole relational half, and everything else the tree needs is inside the
document. That is `docs/03-data-model.md`'s architecture, now written down there explicitly so P1
does not rediscover it.

**Two corrections I made to myself, both from the same root cause.** I read
`render json: JSON.parse(json_data)` in the SSP controller and concluded the API returns OSCAL;
following the call through, `SspDocument#to_json_data` is `{document_name, controls}` — SPARC's
internal shape. The note already in `sparcapi.go` had this right. Then the owner pointed at the
API docs and Postman collection, which showed `?format=oscal` on `cdef_documents` — a parameter I
had not looked for at all, and which narrowed the upstream ask from "build OSCAL export" to
"extend a pattern that already works". **Inferring from one line instead of following the call is
the same mistake as inferring a tool's flags from a call site**, which this log already records
once. Twice now.

**Filed `sparc#1181`** — SSP, SAP, SAR and POA&M have OSCAL export services and `download_oscal`
web routes, but no `/api/v1` surface; the web routes need FIDO2/PIV session auth a service account
cannot hold, and party UUIDs are built *during* export rather than stored, so they cannot be
assembled from other endpoints. `sparc#895` is the accepted precedent for the same shape of gap.
Recorded as X-13.

**P1 is not blocked by it.** The OSCAL SSP fixtures carry the federation and organization parties
with `member-of-organizations`, and `responsible-parties` for AO, system owner and ISSO — exactly
P1's exit criterion. `internal/tree` and `internal/authz` can be built and tested locally; only the
*client* waits on `sparc#1181`. That is the slice to take next, and it was worth establishing
before writing any of it.

**The fixtures are now captured at `?items=5`.** SPARC's real defaults are 25, and 50 for
organizations, which nothing in this federation reaches — so fixtures at the default would be
single-page, and a client that ignored `meta` would pass all of them while truncating a real
federation. Same reasoning as the planted scanner fixtures: a check that cannot fail is not a
check. Three collections now span two pages, and `readPages` in the contract test walks them the
way a client must, asserting the meta adds up rather than trusting it.

**Mutation-checked, both new assertions.** Emitting page 1 only turns the join test red
(`5 rows across pages, meta.count says 7`); re-adding `oscal_party_uuid` turns the new
invented-field test red. The old `TestSPARCRowsCarryTheOSCALUUIDs` asserted the *wrong*
architecture — that boundary rows carry `oscal_ssp_uuid` — so it became
`TestSPARCRowsJoinToTheDocuments`, which asserts the join SPARC can actually serve.

**Next:** **P1** (#16), starting with `internal/tree` over the OSCAL fixtures — the tree builder and
the role mapping, both of which need nothing from SPARC. The client task waits on `sparc#1181`.

---

## 2026-09-23 — #68 — `feature/68_revendor_key_grammar`

**In flight:** nothing.

**SPARC accepted the `family-id` challenge, and Horizon did not notice for a day.** `sparc#1175`
merged as `1cb999b1`: `vocabulary-normalisers` is keyed by vocabulary **and** identifier kind, so
an opaque family is carried as issued. The contract gained two vectors holding
`b9691843-…` and `f2a38fbf-…` — **exactly the identifiers `internal/keys` already derived**, so the
divergence is closed rather than re-specified, and no UUID moved anywhere.
`internal/canonical.FamilyID` needed **no change**: it validated `^[A-Za-z]{2,3}$` then lowercased,
which is equivalent to the contract's "validate `^[a-z]{2,3}$` after lowercasing". Measured, not
assumed — a probe over `AC`/`ac`/`Ac`/`ACM`/`acm` under both vocabularies.

**The real finding is that nothing here could have noticed.**
`TestFamilyIDNormalisationDivergesFromTheContract` kept passing after the disagreement was
settled, because it asserts against the **vendored** copy. Its own failure message anticipated
this — *"either the rule changed here, or the disagreement is resolved and
`docs/dev/sparc-family-id-normalisation.md` is stale"* — and it was the second case, silently.
`TestVendoredContractMatchesItsProvenance` recomputes the digest, so nobody can edit the copy to
make a test pass; that proves it is **unmodified**, not **current**, and the two are easy to
conflate. It surfaced only because I read `sparc` by hand, which is not a process.

**The fix is a scheduled job, deliberately not a required check.** The contract is vendored so the
conformance suite needs no network; putting freshness in the gate would make every PR depend on a
sibling repository being reachable, which is the opposite of the reason for vendoring. So
`vendored-contract-freshness.yml` runs weekly, compares the vendored digest against upstream's
default branch, and **files an issue** — a scheduled job that only fails is one nobody sees. It
dedupes on a marker in the body, because a weekly duplicate trains everyone to ignore it. `sparc`
is **public**, so this needs no credential beyond the default token, used only to open an issue in
*this* repository; nothing writes to a sibling repo. It compares the **file digest, not the
commit**, so unrelated upstream commits do not cry wolf — verified live: upstream `main` has moved
to `bb82c75f` since `1cb999b1` without touching the file, and the check reports current.

**Rejected:** a `shellcheck disable=SC2016` for the markdown backticks in the issue body. The
suppression bar in `issue_rules.md` is high and this did not need to reach it — the body moved to
`.github/issue-templates/vendored-contract-stale.md` and is rendered with `sed`, so there are no
backticks in shell at all, plus an assertion that no `@PLACEHOLDER@` survived rendering. A
template that silently failed to render would file an issue full of placeholders, which reads as
a broken job rather than a real finding.

**Mutation-checked rather than assumed.** Restoring the pre-`1175` behaviour — unconditional
lowercase on an opaque family — turns **three** tests red across two packages, and the one that
catches it in `TestContractVectorsReproduce` is the new `cell-for-foreign-family` vector. Before
the re-vendor every vector used a NIST family, where lowercasing and the scoped rule agree, so
that mutation would have passed the whole vector set. That is the concrete value of re-vendoring,
and it is also exactly how the defect got into the shared contract in the first place.

**Added a floor, not just a non-empty check.** `TestContractVectorsReproduce` asserted only that
the contract carried *some* vectors, so a re-vendor that silently pulled a shorter set would still
report green against whatever remained — the same shape as `ci.yml`'s `MIN_TESTED_PACKAGES`. It
now requires at least 28.

**Next:** **P1** (#16) — SPARC client and tree builder, first Go code, carries S1's scanners, and
inherits TM-11 and #49's measured `go-oscal` limitation. It reads this grammar, which is why this
went first. Otherwise the cleanups: #48 (five `npx` call sites in two required checks), #57 (pin
comments nothing verifies), #42 (Sonar configuration — #63 added that `_test.go` files are held to
a reduced rule set).

---

## 2026-09-22/23 — #63 — `feature/63_sonar_pr_findings`

**In flight:** nothing.

**`hdf fetch` scopes to a pull request, which I had wrongly inferred it could not.** I read the
emit workflow's invocation, saw only `--url --project-key --format --organization`, and concluded
the tool was project-scoped. The owner said otherwise; downloading the pinned binary and running
`--help` showed **both** `--branch` and `--pull-request`. Inference from a call site is not
measurement of a tool, and the cost of the error was proposing a worse design — raw issue JSON
for the development loop instead of the OHDF the question actually asked for.

**`--pull-request`, not `--branch`.** SonarCloud runs *pull request* analysis for these, so a
branch-scoped fetch would look for an analysis that may not exist for a short-lived branch — and
an absent analysis returns an empty result that converts into a clean-looking report, which is
the failure this estate already knows by heart.

**The trap that would have made this quietly useless:** on a `pull_request` event `GITHUB_SHA` is
the **merge commit**, which SonarCloud never analysed. A wait keyed on it times out on every run,
and the tempting fix for a wait that always times out is to delete the wait — which reintroduces
#13. It waits on `pull_request.head.sha`.

**The workflow's first run failed, and both causes were real.** It is its own first exercise, and
that is the only verification that counts for it. What it caught:

*One.* `api/project_analyses/search?…&pullRequest=<n>` **silently ignores the `pullRequest`
parameter** and answers about the default branch. The wait polled for PR #66's head and was handed
`f753a1f` — main's merge of #65. Not an error, not an empty result: a confident answer to a
different question. Had the match been any looser it would have fetched **main's** findings and
filed them as this PR's, which is #13 wearing a new hat. The endpoint that actually reports
per-PR analyses is `api/project_pull_requests/list`, which carries `commit.sha` and
`analysisDate`; it was checked against the live API before being trusted this time.

The date-only fallback went with it. A PR entry exists from its first analysis onward, so matching
on the date alone would have accepted an analysis of an *earlier commit on this same PR*. There is
now no fallback: if the API stops reporting `commit.sha` the job fails loudly rather than
attributing findings to code they were not derived from.

*Two.* **`hdf fetch sonarqube` exits 1 on a clean scan** — `invalid SonarQube structure: missing
or invalid issues field`, when the field is present and simply empty. Reproduced on the pinned
`v3.5.1` and on `v3.7.0`, so it is not a regression. Measured side by side on this project: PR #62
(6 issues) converts, PR #60 and #66 (0 issues) both fail. So the tool cannot be run
unconditionally in CI, which is the only place it earns its keep, and the obvious workaround
(`|| true`) would suppress auth failures and wrong project keys along with the good news. Written
up in [`hdf-cli-empty-sonarqube-result.md`](hdf-cli-empty-sonarqube-result.md); **not filed** —
third party, owner's call, same standing as the `go-oscal` report. The report also carries a
confirmed second defect found in the same runs: rule enrichment 400s because `rules/show` is
called without the `organization` parameter SonarCloud requires, although `--organization` was
supplied.

**Proving the reporting path cost a third measurement.** PR #66 is clean, so its own run
exercises only the zero-finding path. Scratch PR #67 planted a `go:S1192` finding — and the first
attempt reported nothing, because the plant went into a `_test.go` file to protect the coverage
floor. `new_lines` was 18, so the file *was* analysed; `new_violations` was 0. This repository's
`sonar-project.properties` sets `sonar.test.inclusions=**/*_test.go`, and SonarCloud applies a
**reduced rule set** to test sources. Worth knowing for **#42**: test code is held to a smaller
rule set than production code, so a finding class can be absent from `_test.go` without anything
being wrong. Moved to a production file with a test covering it, the full path ran — fetch, one
result rendered, artifact uploaded, and the download parses as OHDF. #67 is closed and deleted.

The workaround is a skip guarded by a count read from `issues/search` directly, marked in the
workflow for removal when the pin can move. That count is not only a guard: it is a **second,
independent source for the number**, and the job now fails if the API reports findings while the
OHDF carries none. The failure this workflow most needs to defeat is an empty result that reads as
a clean report, and until now it had only one place to read that number from.

**Reported, never gated.** The count is printed and never asserted: a PR with no findings is the
normal case, and asserting a non-zero count would make the job fail on good news. What *is*
asserted is that an analysis existed before the fetch. Whether maintainability findings should
fail a PR is a quality-gate change in the console and stays open deliberately.

**Kept off the evidence path.** A separate workflow rather than a `pull_request` trigger on the
emit, with no `id-token` and no bucket. The emit's own header gives two reasons it never runs on
PRs and both still hold; adding a trigger there would put a PR-scoped scan one edit from the
evidence bucket.

**Next:** the remaining cleanups — #48 (five `npx` call sites in two required checks), #57 (pin
comments nothing verifies), #42 (Sonar configuration) — or **P1** (#16), which carries S1's
scanners and inherits TM-11 and #49's measured `go-oscal` limitation as requirements.

---

## 2026-09-22 — #64 — `feature/64_p0_exit_checkpoint`

**In flight:** nothing. **P0's exit checkpoint.** r4 issued, superseding r3.

**The ledger r3 introduced had stopped being kept.** Five firings between r3 and this checkpoint
went unlogged — #37 and #36 on the key grammar, #49 on the trust boundary, #54 on peer
verification and deduplication, #61 as a judgement call — while the table read *"No firing is
currently outstanding."* They were reconstructed from `git log --merges`, which is exactly the
work the ledger exists to avoid. #31 chose checkpointing over per-merge re-attestation **on the
condition** that interim staleness stayed auditable, and that condition was not met.

**An empty ledger and an unmaintained one read identically.** That is the part worth keeping: the
failure is recorded in the ledger rather than corrected quietly, the ledger is now a step-8
obligation in `issue_rules.md`, and a checkpoint reconstructs from the log regardless rather than
trusting the table.

**Two findings added, both measured rather than reasoned.** TM-10 — two *conforming*
implementations derive different identifiers for one object, neither erroring; every one of the
26 shared vectors agreed and the divergence was in a type rule no vector exercises. TM-11 — a
trusted library drops content and validation passes **before and after**, so a pipeline that
validates at both ends sees green twice. Neither was reachable by thinking about the design; both
came from running something against it.

**The trust table gained two rows it should always have had**: the OSCAL type layer and SPARC's
shared key-grammar contract. A threat model that does not name what it trusts is a list of
someone else's problems, and these two were trusted silently.

**A defect of mine surfaced while writing the record.** #54's namespace adoption replaced the
namespace *string* through `docs/compliance/` and never recomputed the *derived identifiers*, so
`compliance/README.md` paired the registered URI with the provisional UUID, and every cdef
identifier still derived from the placeholder. Regenerating them needed the natural keys — and
**the keys had never been written down**. Nine of eighteen fell out of the documented convention
by brute force; the rest had to be named afresh. They are now a table in that README, because a
deterministic identifier whose key exists only in someone's memory is not reproducible, which is
the one property it is for.

**Attestations r1–r3 are left exactly as issued**, under the placeholder namespace. They were
correct when written, and a record edited after the fact is not evidence. Only the current
record's hash binding matches the document, which is how a reader tells which one is current
without consulting the table.

**Next:** P0's exit criteria are all that remain — `sparc-validate` against the fixtures
(`sparc#1154`, upstream and silent since 2026-09-19) and the OpenAPI review by both owners.
Neither is closable from here.

---

## 2026-09-22 — #61 — `feature/61_freeze_api_v0`

**In flight:** nothing. **P0's last task.** Every task in #15 is now done; what remains are the
phase's exit criteria, one of which is upstream.

**The spec was not reviewable, which is why "reviewed by both owners" had never happened.** It
was 120 lines with most responses reading `"200": { description: Ranked controls }`. Freezing
meant writing the contract: schemas for all eleven paths, bodies for the four writes, a shared
error shape, and the refusal rule.

**A refusal is a 404, never a 403.** `docs/07-security.md` already said every node-scoped call
authorizes against the node rather than the endpoint; it never said what the refusal looks like.
A 403 confirms a boundary exists, is called something particular, and by inference who owns it —
across organizations not meant to see each other. The cost, that a mistyped node id reads the
same, is accepted and written down rather than left to be rediscovered as a bug.

**`internal/project` is seeded rather than copied.** `docs/05` gives `StateAt` and `Roll` as
source, and the golden generator needs exactly them. A private copy inside the generator would
have put two implementations of one documented algorithm in the repository — the divergence the
key grammar exists to prevent, one layer up. The package doc names what is P2's and absent,
because that boundary will be under pressure.

**The goldens are built by parsing the OSCAL the same run emitted**, not from the generator's
state. That is the cheap version of the audit claim: if a response cannot be recomputed from
exported documents, it does not belong in the contract either. It also caught a real contract
bug — the `800-53` axis was rendering AWS Security Hub families beside NIST ones, which claims a
crosswalk this repository does not own (`sparc#1103`). They are off the axis now, and the
fixtures still carry them so the distinction stays exercised.

**The check that makes the freeze mean something**: 108 goldens validated against the schema each
path declares, in both directions — the contract cannot declare an endpoint the mock cannot
answer, and the mock cannot serve a shape the contract does not describe. It earned its keep
immediately, failing when the generated README appeared as an unmapped file.

**The mock enforces the 404 rule rather than describing it**, and that is tested through the
router as HTTP responses: an unseen node and an absent one return the same status *and the same
body*, because a difference in either is the leak in another form. A client written against a
lenient mock is a client that breaks on the real service.

**Surfaced, not invented:** `docs/05` weights rollups by FIPS 199 and never gives the weights.
`FIPSWeight` carries a documented default so the mock is reproducible, and `docs/10` now records
it as P2's to settle with users, because the weighting decides which boundary a person is told to
look at first.

**Next:** the **P0 exit checkpoint**. Two criteria left and one is upstream — `sparc-validate`
against the fixtures (`sparc#1154`, no response since 2026-09-19) and the OpenAPI review by both
owners. #31's threat-model staleness ledger folds in at the same point.

---

## 2026-09-22 — #59 — `feature/59_record_upstream_filings`

**In flight:** nothing. **Merge note:** #56 merged first, so both entries sit at the top of this
file; kept in date order per the rule above, this one newer because it records filings made after
that consolidation.

**Three filed upstream, and one of them existed because this table was wrong.**
`sparc#1175` carries the `family-id` disagreement; `sparc-iac#753` carries Horizon's ECS Fargate
runtime; `sparc#1154` got a comment saying the namespace schema it points at moved under it in
#55, and that `fixtures/sparc/` now gives its API-confirmation part something concrete to check.

**`sparc-iac#715` carried two asks and closed on one.** It covered the evidence boundary *and*
the Fargate deployment. The boundary was delivered and proven, the issue closed, and S2-3 went
with it — untracked for three days while the cross-repo table said in bold that nothing was
waiting on an unfiled ask. Nothing was blocked, because there is no image to deploy until S1.
The defect is the assertion, not the delay.

**The pattern worth keeping: a closed issue that carried more than one ask.** The closure is
legitimate, the remainder is invisible, and nothing in the row records that it ever had two
halves. Re-verifying against live issue state rather than against the table is what found it,
and that is now what the claim says it rests on.

**`sparc#1154` has had no response since 2026-09-19.** Not blocking — P0 closed without it — but
its three parts are what P1's client and the axis swap need, so it is watched rather than
assumed.

**Still unfiled, deliberately:** the `go-oscal` report. Third-party, owner's call, unchanged.

**Next:** unchanged — **freeze API v0 and stand up the mock server**, the last P0 task, then the
P0-exit checkpoint that #31 defers the threat-model staleness ledger to.

---

## 2026-09-22 — #56 — `feature/56_dependabot_consolidation`

**In flight:** nothing.

**Three Dependabot PRs consolidated into one, and one of them cannot be taken.**
`golang.org/x/text` v0.42.0 declares `go 1.26.0`; this module pins 1.25.0, `ci.yml` installs
1.25.x and `docs/08` ships `golang:1.25`. Taking it moves the module's Go line, the CI toolchain
and the shipped image together, which is a deployment decision. Closed with that reason and
**no `ignore:` entry** — Dependabot re-raising it weekly is honest, because the bump really is
available and really is blocked on a decision nobody has made, and an ignore entry would hide it
at the moment the Go line finally moves. No security pressure behind it either way: 1.25.13
already carries the fixes #49 found missing in 1.25.0.

**The pin comment beside `setup-go` had been wrong, and Dependabot would have propagated it.**
The SHA labelled `# v6.0.0` is v6.5.0. Dependabot resolves the SHA rather than reading the
comment — its title said "from 6.5.0" — and its diff then carried our wrong comment onto the
v7.0.0 SHA, so merging it unchanged would have pinned v7 while claiming v6.0.0. The comment is
the only human-readable half of a SHA pin, and SA-10's claim that "a bumped pin is a reviewed
diff" rests on it. Audited all five pinned actions: this was the only one lying. Guard filed as
**#57** rather than built here — it needs a token and API calls, which is more than a dependency
PR should carry.

**Next:** unchanged — **freeze API v0 and stand up the mock server**, the last P0 task, then the
P0-exit checkpoint. #55 (the namespace adoption) is open and waiting on review.

---

## 2026-09-22 — #54 — `feature/54_registered_namespace`

**In flight:** nothing.

**`sparc#1155` and `sparc#1161` both landed, and the namespace is not the one we asked about.**
The issue asked SPARC to confirm Horizon's placeholder or name the real one; the answer was
neither. SPARC had already registered `https://sparc.risk-sentinel.org/ns` in its own #1106 —
different host *and* path — so the change belonged here. The federation UUID
`9f434272-f796-589b-b972-954790395630` is **derived** from that URI rather than random, which is
the same construction `ProvisionalNamespace()` used, so adopting it was a constant rather than a
rework.

**The conformance check passed before any of this was written.** Our Go, pointed at the
registered namespace, reproduced all 26 of SPARC's regenerated vectors. That was worth running
first: it meant the reconciliation below was about rules, not about arithmetic.

**Reading their type rules found five divergences that no vector catches.** Four are acceptance
mismatches — period forms, decision dates, control-id prefix length and enhancement depth,
family-id length — and fail loudly. The fifth does not: the contract lowercases `family-id`
unconditionally, so `ACM` under an opaque vocabulary derives one identifier there and another
here, with neither side erroring. **UUID agreement over a fixed corpus is a weak check**; it
only exercises what somebody thought to write a vector for. The conformance test now drives the
contract's own regexes against our canonicalisers instead.

**Four adopted, one held.** We are no longer stricter than the shared grammar, because being
stricter means refusing to ingest an object a peer has already derived an identifier for — which
is why `DecisionDate` now accepts `2026-13-01`, matching a contract that types the field as
`^\d{4}-\d{2}-\d{2}$` and nothing more. The held one is `family-id`: lowercasing a foreign
authority's family is the defect #37 removed from control ids, and
[`sparc-family-id-normalisation.md`](sparc-family-id-normalisation.md) is written and waiting on
the owner, with the looseness above raised as a second, minor point.

**I had the divergence UUIDs wrong in the issue and in the first draft of that document** —
computed under the provisional namespace, before the adoption they were describing. Running the
reproducer rather than trusting what I had written caught it; #54 carries the correction. The
lesson is the one the `go-oscal` report already applied: a quoted value that was never executed
is a guess with good formatting.

**Every identifier in `fixtures/` changed** — 0 of 61 UUIDs in `ssp-ods-portal.json` survived,
which is the check worth running, because a survivor would have meant something was not deriving
under the namespace at all. The 18 AO decisions also moved off the quarter onto the day they
were decided, which is why this could not be split into "adopt the namespace" and "reconcile
later": freezing first would have frozen identifiers the contract calls malformed.

**Attestations were deliberately left alone.** They are dated records bound to a hash, so the
namespace change is a staleness trigger rather than an edit — #31 folds it into the P0-exit
checkpoint, which is now the next thing after the API freeze.

**Next:** **freeze API v0 and stand up the mock server**, the last P0 task. Then the P0-exit
checkpoint: the exit criterion is reachable for the first time, since the identifiers are final.

---

## 2026-09-21 — #49 — `feature/49_oscal_roundtrip_probe`

**In flight:** nothing.

**The re-test found something, which is the argument for having rescoped it.** #15 carried the
#26 obligation as "re-run the probe against the fixture federation". That would have been
circular — `fixtures/oscal/` was written by `go-oscal`, so it contains only fields `go-oscal`
models, and neither thing the probe detects can occur against its own emitter. A green result
there would have been recorded as the measurement that cleared P1. Rescoped to eight published
NIST documents covering all seven models, and filed as #49.

**`go-oscal` collapses four OSCAL assemblies into one Go type, and my first report of it was
too narrow.** OSCAL defines `local-definitions` four times with four different shapes and gives
three of them the same schema title, "Local Definitions". `go-oscal` derives type names from
that title, so the three become one struct carrying the assessment-plan shape. The fourth is a
named `$ref`, which is why `PlanOfActionAndMilestonesLocalDefinitions` exists and the others do
not — the generator's parent-prefix disambiguation only has something to prefix with when the
parent is a named definition.

Two consequences, both now pinned by tests over an authored schema-valid document:
**reading**, a result loses `tasks` *and* `assessment-assets`; **writing**, the same struct
accepts `components`, `inventory-items` and `users` on an assessment-results document, where
OSCAL forbids them — the types produce a document `go-oscal`'s own validator rejects. The
second direction is the one a probe over other people's documents cannot find, because it is
about what Horizon could emit.

A scan of all 41 inline assembly names in the schema found **three** with more than one shape:
`entries` and `status` disambiguate correctly because OSCAL titles them differently;
`local-definitions` is the only collapse. Identical in the OSCAL 1.2.3 schema and in
`go-oscal`'s unreleased 1.2.3 types, so no version bump resolves it.

**The upstream report is written and not filed** —
[`go-oscal-local-definitions.md`](go-oscal-local-definitions.md), with a standalone reproducer
that was run as written rather than retyped into the document. Filing it is the owner's call,
it being a third-party repository. The sharpest line in it is the one found last: the
re-serialised document **still validates**, so a pipeline that schema-validates at both ends
sees green twice and has lost the assessment activities and assets a result recorded.

**The baseline is recorded, not asserted.** `internal/oscal/testdata/measurements.json` holds
every document against every supported version, regenerated with `-update` and reviewed as a
diff. A test written to a guess either fails on a true finding or hides one; this fails when
the measurement *changes*, which is the event worth a person's attention. The one known loss
is additionally pinned by name, so a second gap cannot be absorbed into an updated baseline.

**Cross-version decoding turned out to be a non-issue for this corpus** — every document
decodes identically under all four type packages, so the gap is a missing assembly rather than
version skew. That is a result about this corpus and not a guarantee, which is why
`internal/oscal` still rejects a version it has no types for instead of reaching for the
nearest.

**One of my own tests caught one of my own bugs.** `Model()` was returning whatever single
top-level key a document had, because that is what `go-oscal`'s helper does — right for a
generic tool, wrong for a consumer that acts on the answer. It now rejects a root that is not
an OSCAL model.

**The gate caught a regression I shipped in #36.** `govulncheck` reported three standard-library
vulnerabilities, reachable from this module. Cause: `x/text` v0.41.0 requires Go 1.25.0, so
`go mod tidy` rewrote the directive from `go 1.25` to `go 1.25.0` — and `setup-go` with
`go-version-file: go.mod` and `check-latest: false` then installed **exactly 1.25.0** instead of
the newest patch on the line. Pinning a Go patch pins its known defects, and nothing here would
ever have bumped it: Dependabot does not update a toolchain version in a workflow. `ci.yml` now
pins the **minor** and takes the newest patch, which also matches `golang:1.25` in
`docs/08-build-deploy.md` more closely than the old pin did. Verified: `go1.25.0` reports three
vulnerabilities, `go1.25.13` reports none, and the whole suite — including the fixture
regeneration check — passes under both 1.25.13 and 1.26.3.

**Nothing in CI would have found that.** `govulncheck` is in the verification gate and not in a
workflow; a person running the gate is the only reason it surfaced. That is the argument for
S1's `govulncheck` job, recorded here rather than acted on, because S1 owns it.

**Next:** **freeze API v0 and stand up the mock server** — the last P0 task, unblocked and
needing no decision. Then the P0-exit checkpoint that #31 defers the threat-model staleness
ledger to. `sparc#1155` remains the only thing between the fixtures and the exit criterion.

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
PR** — five call sites, two inside loops that resolve 17 packages per run of a required check.
**Filed as #48**, not folded into #42: that issue is suppression-shaped and lands as
`sonar-project.properties`, while #48 is a true positive with a code fix. A fixtures PR is not
where required checks get rewritten.

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
