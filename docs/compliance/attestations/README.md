# Attestation records

One OSCAL assessment result per revision of the thing attested. A record is **never edited
after it is signed** — a new revision supersedes it, and both files stay.

| Record | Attests | State |
|---|---|---|
| [`threat-model-2026-09-19-r2.oscal.json`](threat-model-2026-09-19-r2.oscal.json) | `../threat-model.md` r2 | **Current.** Expires 2027-03-18 |
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

## Signing, for now

Until phase P4 builds the attestation path, the signature on a record is the **signed commit**
that introduces it. `main` requires verified signatures, so the assertion is cryptographically
bound to the content by the same mechanism that protects every other change here:

```bash
git log --show-signature -- docs/compliance/attestations/
```

**Signing the commit is the act of attesting.** Whoever signs is asserting they performed the
review, so the content is read before it lands.
