# Threat model and security architecture review

**Revision:** r4, 2026-09-22 — the **P0-exit checkpoint**.
**Reviewed revision:** the design of record at `docs/01`–`docs/10` as of 2026-09-22, with P0's
contracts complete: the namespace schema under the registered namespace, the UUIDv5 key grammar
and its Go reference, the fixture federation, and API v0 frozen.
**Attestation:** [`attestations/threat-model-2026-09-22.oscal.json`](attestations/threat-model-2026-09-22.oscal.json),
which supersedes [`threat-model-2026-09-20.oscal.json`](attestations/threat-model-2026-09-20.oscal.json).
**Expires:** 2027-03-18 — **unchanged from r2.** The 180-day clock runs from the substantive
review on 2026-09-19, not from this re-issue: resetting it on each revision would turn a
180-day interval into a perpetual one. Early-staleness triggers are at the bottom of this
document.

> **Why there is already a third revision.** r1 was attested on 2026-09-19 and went stale the
> same day, when #27 added *Signing operates on bytes, not on objects* to
> `docs/06-attestation-workflow.md` — which trips the early-staleness trigger for a change to
> the canonicalisation rule. That produced **TM-9** below and r2. Hours later #30 made the
> UUIDv5 key grammar normative, tripping a second trigger; that firing was recorded rather than
> re-attested, and **r2 was stale from that point**.
>
> r3 closed that gap and wrote down the rule that stops it recurring per-merge — see
> [Freshness](#freshness). Two firings on the first day is the mechanism working, not failing,
> but it is also a signal about cadence during a phase whose entire purpose is to define the
> contracts this document reasons about. The superseded records are kept unmodified rather than
> edited, because an attestation that is quietly revised in place is not evidence of anything.
> See [`attestations/README.md`](attestations/README.md).
>
> **Why there is a fourth.** r4 is the checkpoint r3 promised: P0's contracts are complete, so
> this is one review of a settled design rather than four of a moving one. It folds in five
> firings and adds **TM-10** and **TM-11**, both measured rather than supposed. It also records
> that the interim ledger r3 introduced **was not maintained** — five firings went unlogged
> while the ledger read "no firing outstanding", and were reconstructed from the git log at this
> checkpoint. That failure is in the ledger rather than quietly corrected, because a mechanism
> that silently stopped working is exactly what a reader needs to know about.

This is the one SDLC stage `dev-sec-ops-baseline` deliberately does not automate, because
its output is a document and a conversation. A generated threat model asserts that a
review happened when none did, so this one is written, signed and given an expiry — and
when it expires it reads as expired, which is different from absent.

Horizon has no application source yet. That makes this document unusually load-bearing:
every finding below is a **requirement on the phase that builds the thing**, not an
observation about code someone can go and read. Where a finding is a requirement, the
phase is named.

---

## What is being protected

| Asset | Why it matters |
|---|---|
| The projection answer | Horizon tells an Authorizing Official that a system is authorized through date X. A wrong answer is the product's worst failure, and it is reachable by stale input as easily as by attack |
| The ledger | Append-only and hash-chained. Every materialised projection is rebuildable from it, so it is the single source whose corruption is unbounded |
| Attestations and their signatures | A human assertion that something is true. Forgery here is indistinguishable from diligence |
| Role bindings | Authority comes from `responsible-parties` in the OSCAL documents. Editing a document *is* granting access |
| Evidence objects | The artifacts every claim resolves to |

## What Horizon trusts, and must say so

A threat model that does not state its assumptions is a list of someone else's problems.

| Trusted | Owner | If the assumption fails |
|---|---|---|
| SPARC's catalogs, profiles, crosswalks and pipeline translation | `sparc` | Horizon projects a correct answer about the wrong control |
| The SPARC federation trust fabric and its peer certificates | `sparc` | See TM-6 |
| The agency identity provider | agency | Every identity claim is void; Horizon has no independent account store to fall back to, by design |
| AWS platform controls — task isolation, bucket policy, KMS, secret store | `sparc-iac` | See the inherited rows in [`nist-sp800-53-rev5-mapping.md`](nist-sp800-53-rev5-mapping.md) |
| Image build, scan, SBOM and signature | `container-build-sign` | A compromised image serves correct answers from hostile code |
| The OSCAL type layer, `go-oscal` | third party, pinned | **Known to lose content silently — see TM-11.** It is trusted to represent a document faithfully and does not, for one assembly, at every version |
| The shared key-grammar contract, `sparc:lib/federation/key-grammar.v1.json` | `sparc` | Horizon and a peer derive different identifiers for the same object and neither errors — see TM-10 |

---

## Findings

Each carries a decision: **mitigated** (the design already prevents it), **requirement**
(the design must be built a particular way, and the phase is named), **accepted** (the
risk is understood and carried), or **deferred** (needs a decision that is not this
document's to make).

### TM-1 — Authorization is an ingestion problem, not a login problem

Roles come from `responsible-parties` in the documents rather than from an administration
screen. That is a deliberate and good property: authority is recomputable from an export.
It also means **a document edit is a privilege grant**, and the attack surface for
privilege escalation is the OSCAL ingestion path, not an account system.

An attacker who can introduce or amend an SSP that names their party as a responsible
party for a node acquires authority over that node, and the grant looks exactly like
ordinary document maintenance.

**Decision: requirement (P1, `internal/authz` and `internal/sparc`).**

- Ingestion must verify the bundle's signature before any document is parsed for role
  content, and must refuse rather than partially apply.
- A change to `responsible-parties` must be a first-class ledger event, distinguishable
  in the record from a content edit, so a grant is reviewable after the fact.
- Authorization evaluates the node named in the request, never the endpoint reached. A
  handler that checks the endpoint passes for a node the caller may not read.

**Folded in at the P0-exit checkpoint (#61).** Authorization answers *who may see a node*; it
left open what a refusal discloses. API v0 now fixes that: a node the caller holds no role on
returns **404**, indistinguishable from one that does not exist. A 403 would confirm that a
boundary exists, that it is called something in particular, and by inference who owns it — across
organizations that are not meant to see each other, which is the disclosure this model's trust
boundary exists to prevent. The cost, that a mistyped node id reads the same way, is accepted and
stated in the contract rather than left to be rediscovered as a bug.

---

### TM-2 — What-if isolation is asserted, not yet enforced

`docs/07-security.md` states that overlays are copy-on-write, never persisted, and cannot
emit OSCAL or `saf attest` files, and calls this a security property. Nothing in the
design says what *enforces* it. Overlays deliberately reuse the same projection functions
over an overlay view of the ledger, so the emit and write paths are reachable from
overlay code by construction.

A runtime guard — `if simulated { return }` at the top of the writer — is the obvious
implementation and the wrong one: it is one missed branch away from a simulated
attestation being emitted as real, and the failure is silent and durable.

**Decision: requirement (P2 `internal/ledger` / `internal/project`, P7 what-if).**

- Isolation must be structural. The writer and the emitter take a handle that an overlay
  view cannot produce, so the mistake fails to compile rather than failing in production.
- A test must assert that a simulated context cannot reach the emit path, and it must
  fail if the type separation is later relaxed.
- Any emitted artifact carries its origin, so an escaped simulation is identifiable after
  the fact rather than merely believed impossible.

### TM-3 — The projection can be attacked through its inputs alone

A control reads as down when it fails, when an observation `expires` before the
projection date, or when a POA&M milestone is open before it. Each of those is data.
An attacker who can extend an `expires`, suppress a POA&M item, or delay a scan changes
an AO's answer without touching a line of Horizon's code.

The mirror case is worse than the attack: **ordinary staleness produces the same wrong
answer**, and produces it quietly.

**Decision: requirement (P2 `internal/project`).**

- Expiry is read from the observation's native `expires` field, from the signed document,
  and never from a locally-held or defaulted value.
- Every materialised cell records the evidence it was computed from, so an answer is
  reproducible and attributable rather than merely repeatable.
- Absence of fresh evidence renders as an explicit state, never as a silent hold of the
  last known good value.

**The strongest control here is already in the design:** every cell at every tier must
recompute identically from exported OSCAL alone. A projection that has been tampered with
stops agreeing with its own inputs. Keeping that audit test honest is a security measure,
not only a correctness one.

### TM-4 — The hash chain proves ordering, not truth, and its head is self-published

The ledger is hash-chained and exports carry the chain head. That detects truncation and
post-hoc edits of an export. It does **not** detect a ledger built on false entries, and
it does not detect a wholesale rewrite: an actor who can rewrite the ledger can also
recompute and publish a consistent head, because the head is published by the same party
that holds the ledger.

**Decision: deferred — needs a decision outside this document.**

The chain head needs an anchor the writer does not control. Candidates: emit it to the
evidence bucket under the existing boundary prefix, where the write path is already
role-scoped and independently readable; countersign it through the SPARC federation; or
both. Recorded in `docs/10-risks-decisions.md` as an open phase-0 decision rather than
resolved here, because it constrains the federation contract.

### TM-5 — Signing key compromise forges attestations that recompute correctly

Attestations are detached signatures over JCS-canonical JSON, with the key referenced by
`HORIZON_SIGNING_CERT` and held in the platform secret store. An attacker holding that key
produces attestations that validate, recompute, and are indistinguishable from diligence
by every check Horizon performs.

**Decision: requirement (P4 `internal/attest`), plus one accepted risk.**

- Prefer a signing path where the private key is never in the service's memory — the
  platform key service signs, the service submits a digest. That converts key theft into
  a detectable authorization abuse rather than an undetectable forgery.
- `signed-by` is recorded on every attestation, so revoking a signer identifies exactly
  which attestations fall, over which window.
- **Accepted:** attestations signed before a compromise is discovered remain valid-looking
  until the signer is revoked. The mitigation is the ability to answer *what is the
  posture if this signer is revoked as of date X* — which is precisely a what-if overlay.
  Horizon's own feature is the incident-response tool for this case, and P7 should carry
  that as a named use, not an incidental capability.

### TM-6 — Deterministic UUIDs are predictable, and dedup is by UUID

Object UUIDs are UUIDv5 over natural keys under a single registered federation namespace,
so federated peers deduplicate without coordinating. Since #30 the key grammar is
**normative** rather than illustrative, in `docs/03-data-model.md`, and the roadmap commits
to reference implementations in three languages — Go here in P1, Ruby and Python filed as
`sparc#1161` with a shared test-vector file.

That change strengthened this finding rather than contradicting it: both of the requirements
below are now stated normatively in the data model rather than only here. The finding stands,
because writing a rule down is not enforcing it, and nothing enforces either one yet.

Every peer can therefore compute the UUID of any object in **any** boundary, including
boundaries it does not own. If deduplication is keyed on the UUID alone, a hostile or
compromised peer can precompute the identifier of another boundary's attestation and
submit a document claiming it — and the receiving side treats the collision as the same
object rather than as two different parties' claims about one.

This is not a hash collision. It is the intended, documented determinism being used as
addressing.

**Decision: requirement here, and a cross-repo ask.**

- **Horizon (P6 `internal/federate`):** deduplicate on the pair of object UUID **and**
  originating party, never on the UUID alone. Two peers asserting the same identifier is
  a conflict to surface, not a duplicate to collapse.
- **`sparc`:** the federation trust fabric defines dedup semantics and the key grammar,
  so the estate-wide answer belongs there. Filed — see the cross-repo table in
  `docs/dev/Implementation_plan.md`.

### TM-7 — A stalled pipeline is a silent denial of evidence

Stopping evidence production turns everything amber and then red, which is loud. The
quiet version is the one that has already happened in this estate: `continue-on-error` on
the credential step means a refused write leaves the workflow green, and the GitHub API
reports that step's conclusion as `success` because the flag masks it. An estate-wide
evidence outage ran roughly forty minutes behind green checks on 2026-09-19.

Horizon inherits that pattern in both of its emit paths.

**Decision: requirement (S1), tracking the estate fix.**

- When the estate revisits `continue-on-error` (`sparc-iac#719`), Horizon removes it here
  rather than waiting to be asked; an emit that cannot write must fail the build.
- Until then, no gate in this repository may depend on an emit workflow going red, and
  verification means reading the `upload: … to s3://` line or the object itself.
- Freshness of Horizon's own evidence is itself projectable: the repository is a node, and
  its evidence expiring should read the same way any other node's does.

### TM-8 — Deployment surface

The container is distroless, non-root, a static binary with no shell. The service holds
one AWS identity, the task role. Terraform, the role scope, the bucket policy, the load
balancer and the secret store are owned by `sparc-iac`; build, scan, SBOM and signature by
`container-build-sign`.

**Decision: accepted and inherited.** Recorded so the boundary is visible. Horizon's
obligations are the container and configuration contract in `docs/08-build-deploy.md`, and
`sparc-validate` executing the ECS Fargate and secrets baselines against the deployed
service (X-5).

### TM-9 — A signature fails on a document nobody tampered with

TM-5 is about an attacker producing signatures that verify. This is the opposite failure, and
it is the one that actually happens: a signature that **fails on a document nobody altered**,
because something in the pipeline rewrote bytes without changing meaning.

Measured in #26 rather than supposed. Typed OSCAL carries timestamps as `time.Time`, so
`2026-03-31T19:45:48.195797+00:00` re-serialises as `2026-03-31T19:45:48.195797Z` — the same
instant, different bytes. RFC 8785 canonicalises object member order and number formatting; it
does **not** normalise string values, and a timestamp is a JSON string. So parsing a signed
document into types and serialising it again changes its digest.

The consequence is worse than an error message. On the ingestion path a verification failure
is indistinguishable from tampering and will be treated as an attack, so this defect presents
as a security incident against an honest peer. Under time pressure the tempting fix is to relax
verification, which is precisely the wrong direction.

**Decision: requirement (P4 `internal/attest`, P6 `internal/federate`).**

- Verify and hash over the **bytes as received**. Retain them; do not reconstruct them.
- Parse into types to read and compute. That is what the type layer is for.
- Emit through one canonical serialiser, so a document Horizon authors has one byte form and
  its signature is reproducible by anyone who re-exports it.
- The recompute audit test depends on the same property: if re-export cannot reproduce the
  bytes, a peer cannot verify what Horizon signed.

`docs/06-attestation-workflow.md` carries the rule; this finding is why it is there.

---

### TM-10 — Two conforming implementations can derive different identifiers for one object

TM-6 is about a hostile peer exploiting determinism as addressing. This is the failure with no
attacker in it: two honest implementations, both conforming, deriving **different identifiers for
the same object**, with neither side raising an error.

Measured in #54 rather than supposed. SPARC's shared key-grammar contract normalises `family-id`
by lowercasing it unconditionally; Horizon carries a family from a non-NIST authority as issued,
because canonicalisation is only valid inside a vocabulary — lowercasing the AWS Security Hub
family `ACM` yields `acm`, which names nothing. So a projection cell keyed on that family derives
`b9691843-…` here and `f2a38fbf-…` there.

Both implementations pass the shared vectors. **Every one of the 26 agreed**; the divergence is
in a *type rule* that no vector exercises, and it was found by reading the contract's rules
against the implementation rather than by running its examples.

The consequence is the inverse of TM-6's. There, one identifier is claimed by two parties and the
receiver must not collapse them. Here, one object has two identifiers and the receiver has no way
to know they are the same thing — so the object silently fails to deduplicate, appears twice in a
federated view, and a control that is down in one boundary reads as two unrelated problems.

Deterministic identity only works if determinism is *identical*, and agreement on a corpus of
examples is not agreement on the rules.

**Decision: requirement (P6 `internal/federate`), and an open disagreement.**

- Conformance is asserted against the shared contract's **type rules**, not only its vectors.
  `internal/keys` drives the contract's own regexes against this implementation's
  canonicalisers; that check is what found this.
- The divergence is **pinned by a test** rather than tolerated, so it cannot quietly resolve or
  widen without failing the build.
- Filed as [`sparc#1175`](https://github.com/risk-sentinel/sparc/issues/1175). **Until it is
  settled, do not derive a projection cell identifier for a foreign-vocabulary family against a
  peer** — `docs/03-data-model.md` carries that instruction where an implementer meets it.
- Horizon holds its rule rather than adopting SPARC's, because adopting it would reintroduce for
  families the defect that vocabulary scoping removed from control identifiers.

---

### TM-11 — A trusted library drops content, and validation passes on both sides

TM-9 is a signature failing on a document nobody altered. This is the mirror: a document that is
**altered and still passes every check**.

Measured in #49. `go-oscal` generates one Go type for the four differently-shaped
`local-definitions` assemblies OSCAL defines, because it derives type names from the schema's
`title` and OSCAL reuses `"Local Definitions"` for three of them. A schema-valid
assessment-results document therefore loses `results[*].local-definitions.tasks` and
`.assessment-assets` on the way in — at **every** version from 1.1.2 to 1.2.2, and unchanged in
the unreleased 1.2.3 types.

The sharp part is what happens next: the re-serialised document **validates again**. So a
pipeline that schema-validates its input and its output sees green twice, and has dropped the
assessment activities and assets a result recorded. Nothing in the chain reports anything.

This is a different failure from TM-3. There the input is wrong and the projection faithfully
reflects it; here the input is right and the layer that reads it is lossy. It is also why the
inherited-trust table now names the type layer: Horizon trusts it to represent a document
faithfully, and for one assembly it does not.

**Decision: requirement (P1 `internal/oscal`, P4 `internal/attest`).**

- **Never re-emit a parsed document as though it were the original.** TM-9 already requires
  hashing received bytes, which contains the damage for anything signed; this extends it to
  anything re-exported, signed or not.
- **Schema validation is not a completeness check.** A document that validates before and after
  a round trip may still have lost content, so the audit test is recomputation from the received
  bytes, not revalidation of the output.
- The measurement is a **committed baseline** (`internal/oscal/testdata/measurements.json`) with
  a test that fails when it changes, so a version bump that fixes or widens the gap is a review
  rather than a surprise.
- Reported to the type layer's maintainers rather than worked around —
  `docs/dev/go-oscal-local-definitions.md` carries the report and its reproducer.

---

## Decisions summary

| ID | Surface | Decision | Lands in |
|---|---|---|---|
| TM-1 | Authorization via document ingestion | Requirement | P1 |
| TM-2 | What-if isolation | Requirement — structural, not a runtime guard | P2, P7 |
| TM-3 | Projection inputs and staleness | Requirement | P2 |
| TM-4 | Chain head has no independent anchor | Deferred — open decision | `docs/10-risks-decisions.md` |
| TM-5 | Signing key compromise | Requirement + one accepted risk | P4, P7 |
| TM-6 | Predictable UUIDs used as addressing | Requirement + `sparc` ask | P6, cross-repo |
| TM-7 | Silent evidence stall | Requirement | S1 |
| TM-8 | Deployment | Accepted, inherited | `sparc-iac`, `container-build-sign` |
| TM-9 | A signature fails on an untampered document | Requirement | P4, P6 |
| TM-10 | Conforming implementations derive different identifiers | Requirement + open disagreement (`sparc#1175`) | P6, cross-repo |
| TM-11 | A trusted library drops content, and validation passes | Requirement | P1, P4 |

Nothing here is marked mitigated. That is the correct result for a repository with no
application code: the design is sound on paper, and none of it is enforced yet.

TM-9 arrived after r1 was signed, from measurement rather than review. That is the expected
way for this document to grow: a spike measures something, the measurement contradicts an
assumption, and the model is re-issued rather than amended in silence.

---

## Freshness

**Interval:** 180 days. This attestation expires **2027-03-18**.

It goes stale **early**, regardless of the date, on any of:

- a change to the trust boundary — what Horizon trusts, or who may assert into it;
- a change to the evidence chain order, the signing scheme, or the canonicalisation rule;
- a change to how roles are derived from documents;
- what-if overlays gaining any ability to write or emit;
- a federation change affecting peer verification, deduplication, or the UUID key grammar;
- the first release that serves real projections to an Authorizing Official.

A phase completing is not by itself a trigger. Building what this document requires does
not invalidate it; changing what it assumed does.

### Cadence during a contract-defining phase

**While a phase whose deliverables are the contracts this document reasons about is open, a
trigger firing is logged in the ledger below and the attestation is re-issued at phase exit,
not per merge.** Phase P0 is such a phase by construction: its deliverable list is the
namespace schema, the UUIDv5 key grammar, the fixture federation and the frozen API, so the
trigger for "a federation change affecting peer verification, deduplication, or the UUID key
grammar" fires for most of the phase.

The triggers themselves are **not narrowed**. Narrowing them to "a change that contradicts a
finding" would replace a bright line with a judgement call, and the person making it is the
one who would rather not re-attest. The line stays where it is; what changes is when the
re-review happens, and the interim is recorded rather than silent.

Two limits keep this from becoming an excuse:

- **It applies only while such a phase is open.** Outside one, a trigger means a re-issue.
- **A firing that contradicts a finding or invalidates a stated assumption re-issues
  immediately, phase or not.** The ledger is for firings that touch an area without changing
  what was concluded. #27 would not have qualified — it produced TM-9.

Between checkpoints this document reads as **stale in the logged areas**, and the ledger says
which. That is the cost, and it is stated here so a reader is not misled into treating the
record as continuously current.

### Interim staleness ledger

Every trigger firing and what became of it. A firing is **logged** until a checkpoint folds it
in, at which point it is marked with the revision that absorbed it. Nothing is removed — the
record of a gap is the point of the ledger, and a reader needs to see that the gap existed and
how long it lasted.

| Date | Change | Trigger | Contradicted a finding? | Disposition |
|---|---|---|---|---|
| 2026-09-19 | #30 — the UUIDv5 key grammar made normative | Federation change affecting peer verification, deduplication, or the UUID key grammar | **No.** It strengthens TM-6: the party-not-in-key rule and consumer-side deduplication are now normative in `docs/03-data-model.md` | **Folded into r3** (#31). r2 read as stale in this area from 2026-09-19 until 2026-09-20 |
| 2026-09-20 | #37 — `source-uuid` added to seven field lists; control-id canonicalisation scoped to a vocabulary | Federation change affecting the UUID key grammar | **No.** It partitions the key space by catalog authority, which TM-6 assumed rather than required | **Logged late, at the P0-exit checkpoint. Folded into r4** |
| 2026-09-21 | #36 — the Go reference implementation of the grammar, and the fixture federation | Federation change affecting the UUID key grammar | **No.** First implementation of a rule the document already carried | **Logged late. Folded into r4** |
| 2026-09-21 | #49 — `go-oscal` measured to drop two fields from a schema-valid assessment-results document, silently, in both directions of validation | Trust boundary — what Horizon trusts | **No, it extends.** TM-9 already required hashing received bytes; nothing said a trusted dependency loses content | **Logged late. Folded into r4 as TM-11**, and the type layer added to the trust table |
| 2026-09-21 | #54 — the registered federation namespace adopted; **every identifier in the estate changed**; a cross-implementation divergence with SPARC on `family-id` | Federation change affecting peer verification, deduplication, or the UUID key grammar | **No, it extends.** TM-6 covers a hostile peer claiming an identifier; two honest peers deriving different identifiers for one object was not considered | **Logged late. Folded into r4 as TM-10** |
| 2026-09-22 | #61 — API v0 frozen, with an unauthorized node returning 404 rather than 403 | **Judgement call.** Not a change to who may assert into the boundary, so arguably no trigger; logged rather than argued away, because the triggers are deliberately not narrowed | **No.** It settles a disclosure question TM-1 left open | **Logged. Folded into r4** as a note on TM-1 |

### What went wrong with this ledger, recorded rather than fixed quietly

Between r3 and the P0-exit checkpoint, **five firings went unlogged for up to two days**, while
the table above read *"No firing is currently outstanding."* The entries were reconstructed from
the git log at the checkpoint — which is precisely the work the ledger exists to avoid.

#31 chose checkpointing over per-merge re-attestation, and the condition it chose it on was that
interim staleness would stay auditable. A ledger written only when someone remembers does not
meet that condition; it is worse than no ledger, because it reads as evidence of nothing having
happened.

Two things follow, and they are process rather than design:

- **The ledger is a step-8 obligation**, beside the session log and the implementation plan, in
  the PR that trips the trigger. `docs/dev/issue_rules.md` now says so. It is cheap in the PR
  that caused it and expensive at a checkpoint.
- **A reader should distrust an empty ledger.** "No firing outstanding" means the same thing
  whether it is true or unmaintained, so the checkpoint reconstructs from the log regardless
  rather than taking the table's word for it.

No firing is currently outstanding **as of r4**, and that statement was checked against
`git log --merges` rather than against this table. The next checkpoint is **P1 exit**, or any
trigger firing that contradicts a finding, which re-issues immediately.

## How this is attested

Each revision of this document gets its own OSCAL assessment result, with a single
observation carrying a native `expires`, so Horizon's own review is countable by the same
engine that counts everyone else's — the evidence chain in `docs/06-attestation-workflow.md`, applied
to this repository.

The signature is the **signed commit** that introduces the attestation. `main` requires
verified signatures, so the reviewer's assertion is cryptographically bound to the
content by the same mechanism that protects every other change to this repository, and is
verifiable with `git log --show-signature`. When P4 builds the attestation path, this
record is re-issued through it and the commit signature becomes the corroborating
evidence rather than the primary one.

A superseded record's hash identifies a revision of this file that no longer sits at this
path. That revision is not lost: `main` requires verified signatures, so git history is the
immutable store, and the superseded hash resolves against it. This is why a superseded record
is never edited — editing it would break the one link back to what was actually reviewed.
