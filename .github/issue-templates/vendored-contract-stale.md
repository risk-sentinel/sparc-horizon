@MARKER@

`@PROVENANCE@` pins `@UPSTREAM_PATH@` at `@PINNED@`, and upstream `@BRANCH@` has moved.

| | digest |
|---|---|
| vendored | `@HAVE@` |
| upstream `@SHA@` | `@WANT@` |

The conformance test in `internal/keys` asserts against the **vendored** copy, so it cannot see
this by construction. That is why a scheduled job reports it rather than a required check — the
contract is vendored precisely so the gate needs no network (#68).

To resolve: re-vendor `@UPSTREAM_PATH@` from `@UPSTREAM_REPO@`, update `PROVENANCE.json` (commit,
retrieved, sha256, bytes), and run the `internal/keys` suite. If a vector disappeared upstream,
the count floor in `TestContractVectorsReproduce` will say so rather than silently proving less.

Opened by `.github/workflows/vendored-contract-freshness.yml`.
