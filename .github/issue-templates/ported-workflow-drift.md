@MARKER@

The files ported from `@UPSTREAM_REPO@` at `@PINNED@` could not be confirmed current against its default branch.

@DETAIL@

**Upstream moved:** diff the listed files between `@PINNED@` and the upstream commit shown. Re-port the changes, keeping every `PORT(sparc-horizon)` edit. Then update `.github/ported/PROVENANCE.json` (commit and each `upstream_sha256`) and run the SCA fixture workflow.

**Unverifiable:** `@UPSTREAM_REPO@` is internal, so this job needs `CBS_READ_TOKEN`, a read-only token for that repository, until `container-build-sign#342` makes it public. After that the default token is enough, and the secret can be deleted.

Opened by `.github/workflows/ported-workflow-drift.yml`.
