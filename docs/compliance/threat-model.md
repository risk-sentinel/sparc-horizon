# Threat model and security architecture review

**Revision:** r2, 2026-09-19.
**Reviewed revision:** the design of record at `docs/01`–`docs/10` as of 2026-09-19, including
the signing rule added by #27.
**Attestation:** [`attestations/threat-model-2026-09-19-r2.oscal.json`](attestations/threat-model-2026-09-19-r2.oscal.json),
which supersedes [`threat-model-2026-09-19.oscal.json`](attestations/threat-model-2026-09-19.oscal.json).
**Expires:** 2027-03-18. Early-staleness triggers are at the bottom of this document.

> **Why there is already a second revision.** r1 was attested on 2026-09-19 and went stale the
> same day, when #27 added *Signing operates on bytes, not on objects* to
> `docs/06-attestation-workflow.md` — which trips the early-staleness trigger for a change to
> the canonicalisation rule. That produced **TM-9** below. The superseded record is kept
> unmodified rather than edited, because an attestation that is quietly revised in place is not
> evidence of anything. See [`attestations/README.md`](attestations/README.md).

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
so federated peers deduplicate without coordinating. The key grammar is documented and
the roadmap commits to publishing reference implementations in three languages.

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
