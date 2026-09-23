# `hdf fetch sonarqube` fails on a project with no issues

**Status: written, not filed** (as of 2026-09-23). This is a defect in a third-party tool
([`mitre/hdf-libs`](https://github.com/mitre/hdf-libs)), not in this estate. Filing it upstream is
the repository owner's call, so it is recorded here with its evidence rather than sitting in
someone's notes. Everything below the line is written to be filed as-is.

Found while building the pull-request findings workflow
([`.github/workflows/sonar-pr-findings.yml`](../../.github/workflows/sonar-pr-findings.yml), #63):
the workflow's own first run hit it, because the pull request that introduced it was clean.
The workaround carried in that file is a skip guarded by an independent issue count, and it is
marked for removal once this is fixed.

---

## Summary

`hdf fetch sonarqube` exits 1 when the SonarQube/SonarCloud project, branch, or pull request has
**zero issues**:

```
Error: sonarqube conversion failed: invalid SonarQube structure: missing or invalid issues field
```

The message is wrong in both of its claims. The `issues` field is neither missing nor invalid —
the API returned it, correctly, as an empty array:

```json
{ "total": 0, "p": 1, "ps": 1, "paging": {"pageIndex":1,"pageSize":1,"total":0},
  "issues": [], "components": [], "facets": [] }
```

An empty result set is the **normal, desirable** state of a clean codebase. A converter that
fails on it cannot be run unconditionally in CI, which is precisely where it is most useful.

## Affected

| | |
|---|---|
| Tool | `hdf` (`mitre/hdf-libs`) |
| Versions confirmed | `v3.5.1` (commit `7c868e99`) and `v3.7.0` (commit `537505a0`) |
| Command | `hdf fetch sonarqube` |
| Server | SonarCloud, reporting version `8.0.0.107671` |
| Scope | `--pull-request`; by inspection any scope whose issue set is empty |

Both the oldest version this project pins and the newest release exhibit it, so this is not a
recent regression.

## Reproducer

Any Sonar project with no open issues in scope. Against a pull request:

```bash
export SONARQUBE_TOKEN=<token>

# First, show the API returns a well-formed response with an empty issues array:
curl -s -u "${SONARQUBE_TOKEN}:" \
  "https://sonarcloud.io/api/issues/search?componentKeys=<project-key>&pullRequest=<n>&ps=1" \
  | jq '{total, has_issues_field: has("issues"), issues}'
# => { "total": 0, "has_issues_field": true, "issues": [] }

# Then convert the same scope:
hdf fetch sonarqube out.json \
  --url https://sonarcloud.io \
  --project-key <project-key> \
  --organization <org> \
  --pull-request <n> \
  --format hdf
# => Error: sonarqube conversion failed: invalid SonarQube structure: missing or invalid issues field
# => exit 1
```

Measured side by side on two pull requests of the same project, same binary, same credentials:

| Pull request | `issues/search` `total` | `hdf fetch sonarqube` |
|---|---|---|
| #62 | 6 | exit 0, valid OHDF, 6 results |
| #60 | 0 | exit 1, `missing or invalid issues field` |
| #66 | 0 | exit 1, `missing or invalid issues field` |

The only difference between the passing and failing runs is whether the project had findings.

## Why it matters

The whole point of converting scanner output to OHDF is to put it on an evidence path that runs
every time. Three consequences follow from failing on the empty case:

1. **A clean scan is indistinguishable from a broken one.** Both exit 1. Any caller that treats a
   non-zero exit as "the pipeline is broken" pages someone because the code is good.
2. **It pushes callers toward suppressing the exit status.** The obvious fix is `|| true`, which
   also suppresses authentication failures, wrong project keys, and network errors — every case
   where an empty result genuinely *is* wrong. The defect encourages the mitigation that destroys
   the signal.
3. **It cannot produce the negative evidence.** An OHDF profile with zero results is a meaningful
   artifact: it asserts that a scan ran against this revision and found nothing. Today that
   assertion cannot be emitted at all, so "scanned and clean" leaves no record.

## Suggested fix

Treat a present-but-empty `issues` array as valid input and emit an OHDF profile with an empty
`controls`/`results` set, preserving the `generator`, `tool` and `timestamp` metadata that make it
evidence.

Distinguish the three states the current code conflates:

| API response | Correct behaviour |
|---|---|
| `issues` absent | Error — the payload is malformed |
| `issues: []` | Success — emit OHDF with no results |
| `issues: [...]` | Success — emit OHDF with results (today's behaviour) |

If a caller genuinely wants "fail when there is nothing to convert", that belongs behind an
explicit flag, not as the default.

## Secondary observation: rule enrichment fails for Go rules

Not the subject of this report, but observed in the same runs and probably worth its own issue.
On the pull request that *did* convert, every rule lookup failed:

```
WARNING: failed to enrich rule go:S3776: rules/show API returned HTTP 400 for go:S3776
WARNING: failed to enrich rule go:S1192: rules/show API returned HTTP 400 for go:S1192
```

The conversion succeeded and the results carry their messages, so the impact is reduced
descriptive metadata rather than lost findings.

The cause is confirmed: SonarCloud's `rules/show` **requires** an `organization` parameter, and
the call is being made without it, although `--organization` was supplied to the command.

```bash
curl -s -u "${SONARQUBE_TOKEN}:" "https://sonarcloud.io/api/rules/show?key=go:S3776"
# 400 {"errors":[{"msg":"The 'organization' parameter is missing"}]}

curl -s -u "${SONARQUBE_TOKEN}:" \
  "https://sonarcloud.io/api/rules/show?key=go:S3776&organization=<org>"
# 200 {"rule":{"key":"go:S3776","name":"Cognitive Complexity of functions should not be too high",
#              "type":"CODE_SMELL","severity":"CRITICAL", ...}}
```

Threading the already-supplied organization through to the `rules/show` call should be sufficient.

## Environment

| | |
|---|---|
| `hdf` | `3.5.1` (`7c868e99`) and `3.7.0` (`537505a0`, built 2026-09-22, go1.26.6) |
| Platform | `darwin/arm64`. Not platform-specific by inspection — the failure is in the conversion path, not in I/O — but only `darwin/arm64` was measured |
| Server | SonarCloud, `8.0.0.107671` |
| Auth | Basic, token-as-username |
