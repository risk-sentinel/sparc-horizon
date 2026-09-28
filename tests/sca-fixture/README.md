# SCA detection fixture

`vulnerable-components.json` is a CycloneDX SBOM that names two packages at versions with
published **CRITICAL** advisories:

| Component | Version | Advisory |
|---|---|---|
| `golang.org/x/crypto` | v0.30.0 | CVE-2024-45337 (GHSA-v778-237x-gjrc) |
| `lodash` | 4.17.11 | CVE-2019-10744 (GHSA-jf85-cpcp-j695) |

`.github/workflows/sca-fixture.yml` feeds it to the SCA gate and asserts that the gate finds
them. A scanner that stops matching then fails a check, instead of passing every real scan
with nothing found. This is the same idea as the TruffleHog fixture under
`tests/trufflehog-fixture/`.

Nothing here is a dependency of this repository. The filename matches none of Syft's SBOM
patterns (`*.cdx.*`, `*.bom.*`, `*.sbom.*`, `*.spdx.*`, `bom`, `sbom`), so the real source SBOM
does not absorb these components. If this file is renamed to one of those patterns, the main
gate fails on it at once, which makes the mistake visible rather than silent.
