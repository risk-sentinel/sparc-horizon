# actionlint shellcheck canary

`shellcheck-canary.yml` is a **deliberately defective** workflow file. It is never executed.

## Why

`actionlint` finds shell defects inside `run:` blocks by shelling out to `shellcheck`. When
the `shellcheck` binary is not on `PATH`, actionlint **skips that integration and exits 0**,
with no warning. A green `Workflow lint` job therefore cannot, on its own, distinguish:

- every workflow's shell is clean, from
- shellcheck was never consulted

`ubuntu-latest` ships shellcheck today. Nothing guarantees it always will, and a runner image
change would remove the coverage silently.

## How it is used

`.github/workflows/contracts.yml`'s `Workflow lint` job:

1. asserts `shellcheck` is on `PATH`, with an error naming the fix if not
2. runs `actionlint` against **this file only** and fails unless `SC2012` is reported
3. lints the real workflows in `.github/workflows/`

Step 2 is the load-bearing one. Step 1 proves the binary exists; only step 2 proves
actionlint is using it.

## Measured

| actionlint | shellcheck | Result on this file |
|---|---|---|
| v1.7.12 | 0.10.0 on `PATH` | `rc=1`, reports `SC2012` |
| v1.7.12 | absent | `rc=0` |

## Do not fix the defect

The `ls -1 /tmp/*.txt | wc -l` in the fixture is the plant — `SC2012` ("use find instead of
ls"). Correcting it disables the canary while leaving the job green, which is precisely the
condition the canary exists to detect.

This file is outside `.github/workflows/` on purpose: `actionlint` with no arguments lints
`.github/workflows/` only, so the real pass ignores it.
