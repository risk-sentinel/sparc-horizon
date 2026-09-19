# Secret-scanning detection fixture

This directory contains a **deliberately planted, non-functional** credential. It
exists so the secret-scanning gate can prove it still works on every run.

## Why

Without a planted finding, "no secrets found" and "the scanner did not run" produce
byte-identical output. A scanner that regressed — a bad pin, a changed flag, a
detector that stopped firing — would report green forever, and nothing downstream
could tell.

`.github/workflows/secret-scan.yml` therefore has two jobs:

| Job | Scans | Mode | Meaning of failure |
|---|---|---|---|
| `scan` | the repository, excluding this directory | `--only-verified` | A real, verified credential is present |
| `fixture-detection` | **only** this directory | default (unverified included) | The scanner is broken — the gate above cannot be trusted |

## The plant

`aws-credentials.txt` holds the AWS documentation example key pair
(`AKIAIOSFODNN7EXAMPLE`). It is published by AWS as a documentation placeholder, is
not a credential for any account, and authenticates to nothing. It is used here
precisely because it is universally recognisable as fake while still matching the
`AKIA`-prefixed shape TruffleHog's AWS detector fires on.

**Do not put a real credential here, ever, including an expired or rotated one.**
A rotated key is still a disclosure of an account identifier, and this repository's
history is permanent.
