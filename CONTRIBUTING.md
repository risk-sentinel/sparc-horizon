# Contributing to SPARC Horizon

Thanks for working on Horizon. This document covers the pull-request conventions
so the PR Checklist gate does not bite you.

For the broader issue, branch, verification, and compliance workflow, see
[`docs/dev/issue_rules.md`](docs/dev/issue_rules.md) — it is the canonical process
and is **mandatory** for non-trivial work.

## Before you start

- Horizon is part of the **Risk-Sentinel** estate. It reads its sibling
  repositories and writes to none of them. Work that belongs elsewhere is filed as
  an issue there — see the ownership table in `issue_rules.md`.
- **Security work precedes feature work.** The current phase is the security and
  supply-chain baseline in
  [`docs/dev/Implementation_plan.md`](docs/dev/Implementation_plan.md).
- **No AI-assistant attribution** in commits, PR titles or bodies, comments, code,
  or docs. A tool default that appends one must be overridden.

## Pull request convention

`.github/workflows/pr-checklist.yml` runs on every PR and fails if **any unchecked
checkbox (`- [ ]`) remains in the PR body**. That is intentional for the
human-verified test plan, but it bites two legitimate cases:

1. Items that can only be confirmed by CI runs on the PR itself
2. Items verified after merge

So PRs use a **fixed five-section shape** — checkboxes belong to exactly one
section, the others use plain bullets.

```markdown
## Summary                  — 1-3 sentences, what and why
## Changes                  — bullet list of concrete diffs
## Test plan                — `- [ ]` checkboxes; ALL checked before merge
## Verified by CI           — plain bullets, no checkboxes
## Post-merge verification  — plain bullets, no checkboxes
## Notes                    — optional context
```

`.github/PULL_REQUEST_TEMPLATE.md` ships this shape, so opening a PR
pre-populates it.

If you genuinely need checkboxes outside the Test plan, wrap them:

```markdown
<!-- pr-checklist:skip -->
- [ ] something CI will confirm
<!-- /pr-checklist:skip -->
```

## Record measurements, not verdicts

The Test plan is read as evidence. "Tests pass" is a verdict; a measurement names
the command, the count, and the version:

```
go test ./... -race — 42 tests, 18 packages, 71.4% coverage
govulncheck ./... — 137 modules analysed, 0 vulnerabilities
TruffleHog 3.97.4 — 0 verified findings; fixture-detection FAILED as expected
```

This matters because several of the commands involved report success while having
assessed nothing. `go test` on a package with no tests exits 0 and prints
`no test files`. A scanner pointed at an unknown project returns an empty result
that converts into a clean report. The counts are what distinguish those from a
real pass — see the measurement rules in `issue_rules.md`.

## Branching

- `feature/<issue>_slug` or `bug/<issue>_slug`, with the issue number in the name
- One PR per issue; multiple commits per PR is fine
- Never push to `main`. Only the owner merges — including their own PRs
- **If you are not on `main`, ask before cutting another branch.** A second branch
  from `main` gets a stale copy of every shared process file, and an edit against a
  stale copy has to be redone rather than cherry-picked. Supporting changes ride
  the open bundle's branch

## Suppressing a scanner finding needs approval first

Every suppression is a risk-acceptance decision and belongs to the owner, not to
whoever is looking at the red check. Fix first; if a rule genuinely cannot be
satisfied, demonstrate that against the pinned toolchain rather than asserting it.
Call any suppression out in the PR body under `## Notes` so it is reviewed as a
decision rather than skimmed as noise. The full bar, and what counts as a
suppression, is in `issue_rules.md`.

Deleting or narrowing a test so it stops failing is a suppression too.

## Things that will fail your PR

| Symptom | Cause |
|---|---|
| `Test plan checklist` red | An unchecked `- [ ]` outside a skip block |
| `Verified secrets gate` red | A verified credential in the tree |
| `Fixture detection` red | The secret scanner regressed — the gate above it cannot be trusted |
| `Duplicated copies agree` red | `docs/hud.html` drifted from `demo/hud.html`, or `demo/full-plan.html` no longer carries a demo script or a roadmap phase |
| `Workflow lint` red | A workflow schema error. Left unlinted these produce a 0-second run with no jobs and no logs |
| `Namespace schema` red | A documented prop example no longer validates, or the schema started accepting something it should reject |
