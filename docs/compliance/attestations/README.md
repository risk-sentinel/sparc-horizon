# Attestation records

One OSCAL assessment result per revision of the thing attested. A record is **never edited
after it is signed** — a new revision supersedes it, and both files stay.

| Record | Attests | State |
|---|---|---|
| [`threat-model-2026-09-22.oscal.json`](threat-model-2026-09-22.oscal.json) | `../threat-model.md` r4 | **Current.** The P0-exit checkpoint. Expires 2027-03-18 |
| [`threat-model-2026-09-20.oscal.json`](threat-model-2026-09-20.oscal.json) | `../threat-model.md` r3 | **Superseded** by r4 |
| [`threat-model-2026-09-19-r2.oscal.json`](threat-model-2026-09-19-r2.oscal.json) | `../threat-model.md` r2 | **Superseded** by r3 |
| [`threat-model-2026-09-19.oscal.json`](threat-model-2026-09-19.oscal.json) | `../threat-model.md` r1 | **Superseded** by r2, same day |

## Why a superseded record is kept, and kept unmodified

Each record binds itself to the exact text it reviewed by SHA-256. When the document is
revised, the superseded record's hash no longer matches the file at that path — and that is
the correct behaviour, not a broken link. It identifies the revision that was **actually
reviewed**.

That revision is recoverable: `main` requires verified signatures, so git history is the
immutable store here, and the hash resolves against it:

```bash
# find the revision a superseded record attests
git log --oneline --all -- docs/compliance/threat-model.md
git show <commit>:docs/compliance/threat-model.md | shasum -a 256
```

Editing a superseded record to "fix" the hash would destroy the only link back to what was
reviewed, and would make every other record untrustworthy by implication — if one was quietly
revised, any of them could have been. So they are not edited.

## Naming

`<subject>-<review-date>[-r<n>].oscal.json`. The first record of a date carries no suffix;
subsequent revisions of the same review are `-r2`, `-r3`. The date is the date of the
**review**, not of the file.

## Why r2 exists hours after r1

r1 was attested on 2026-09-19. Later the same day, #27 added *Signing operates on bytes, not
on objects* to `docs/06-attestation-workflow.md`, which trips r1's own early-staleness trigger
for a change to the canonicalisation rule. The re-review added **TM-9** and produced r2 (#28).

This is the freshness mechanism working on the first occasion it fired. An attestation that
goes stale and is quietly left in place is worse than no attestation, because it reads as
current.

## Why r3 is dated a day later, and carries no suffix

The date in a filename is the date of the **review**, not of the file, and a `-r<n>` suffix
marks a further revision *of the same review*. r1 and r2 were two revisions of the 2026-09-19
review, so r2 took the suffix. r3 is a new review, performed on 2026-09-20, so it is the first
record of that date and takes none. The document it attests is still called revision 3, because
that counts revisions of the threat model rather than records of a review.

r3 exists for two reasons at once. #30 made the UUIDv5 key grammar normative on 2026-09-19,
tripping the trigger for a federation change affecting the key grammar; that firing was logged
rather than re-attested, so **r2 read as stale in that area for a day**. And the fix for that
pattern — re-issuing at phase exit rather than per merge, in #31 — is itself written in the
attested document, so recording it changes the document's hash and requires a new record. The
rule that reduces re-attestation could not be adopted without one more re-attestation.

Note what r3 is **not**: a reset of the expiry. It keeps 2027-03-18, which is 180 days from the
substantive review on 2026-09-19. Resetting the clock on every revision would turn a bounded
interval into a perpetual one.

## Signing, for now

Until phase P4 builds the attestation path, the signature on a record is the **signed commit**
that introduces it. `main` requires verified signatures, so the assertion is cryptographically
bound to the content by the same mechanism that protects every other change here:

```bash
git log --show-signature -- docs/compliance/attestations/
```

**Signing the commit is the act of attesting.** Whoever signs is asserting they performed the
review, so the content is read before it lands.

## Identifiers, and why the older records look different

Every UUID in **r4** derives under the registered federation namespace,
`9f434272-f796-589b-b972-954790395630` (`sparc#1155`, adopted in #54), over the natural keys
recorded in [`../README.md`](../README.md).

**r1 through r3 derive under Horizon's placeholder namespace** and are left exactly as issued.
They were correct when written, the namespace they used was the one this repository had, and an
attestation edited after the fact is not evidence of anything — which is the rule this directory
exists to enforce. So the identifier scheme changes at r4 and the older records do not move.

A reader comparing two records across that boundary should expect the party and resource UUIDs to
differ even where they name the same thing. The hashes are what bind a record to its document,
and those are unaffected.
