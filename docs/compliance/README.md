# Compliance artifacts

Horizon's own control story. This directory is the answer to the question Horizon
asks every system it projects: *show me the evidence*.

| File | What it is |
|---|---|
| [`nist-sp800-53-rev5-mapping.md`](nist-sp800-53-rev5-mapping.md) | The control table — status, implementation, evidence, code location |
| [`oscal/cdefs/`](oscal/cdefs/) | OSCAL component definitions, the machine-readable form of the same claims |
| [`threat-model.md`](threat-model.md) | The threat model and its attestation (#12) |

## The rule this directory exists to enforce

**A control claim is worth exactly what the evidence behind it says, and nothing more.**

Horizon's product thesis is that a control's state is what can be recomputed from
exported OSCAL — not what a document asserts. A compliance directory that claims
controls Horizon cannot demonstrate would refute the product in its own repository.
So every row in the mapping carries an evidence column, and a row with no evidence
is marked `planned`, never `implemented`.

## Three layers, kept apart

Horizon is an application on a platform it does not own. Conflating the layers is how
these documents turn into fiction, so each row states which layer it belongs to.

| Layer | Owner | What Horizon may claim |
|---|---|---|
| **Authored** — authorization per node, the hash-chained ledger, evidence signing, what-if isolation, input validation | Horizon | Everything. This is the set Horizon implements and must evidence |
| **Pipeline** — secret scanning, static analysis, dependency updates, branch protection, signed commits, evidence emission | Horizon, via `dev-sec-ops-baseline` reusables | The configuration and its evidence. The scanners themselves are upstream |
| **Inherited** — ECS Fargate, the evidence bucket, KMS, ECR, the load balancer, the secret store | `sparc-iac`, `container-build-sign` | Nothing directly. Horizon records the boundary and what it assumes of the platform |

`issue_rules.md` asks for maximum documented **application-layer** coverage. The
inherited rows are there so an assessor can see where Horizon's responsibility stops —
not so Horizon can count someone else's controls as its own.

## Leveraging the AWS service component definitions

[`awslabs/oscal-content-for-aws-services`](https://github.com/awslabs/oscal-content-for-aws-services)
publishes OSCAL component definitions for 230 AWS services, Apache-2.0, OSCAL 1.2.1 —
the same licence this repository carries. The inherited layer imports them by reference
rather than describing ECS or S3 in prose written here, which keeps the platform
description maintained by the party that owns the platform.

**Its control implementations are keyed to AWS Security Hub control IDs** (`ECS.1`,
`S3.5`), against its own Security Hub catalog. There is no NIST 800-53 linkage in that
content: a control there carries a severity, a trigger type and a Config rule id, and
nothing that names an 800-53 control. So the import supplies **component identity and
description**, not control satisfaction.

Crossing Security Hub to 800-53 rev 5 is a **mapping, and mappings belong to `sparc`**
(`docs/01-scope.md`). Horizon must not author one here — and does not need to: SPARC
already ships an AWS Security Hub to NIST 800-53 rev 5 converter, alongside runtime
ingestion of the AWS Labs component definitions. The attribution is *resolved upstream
and consumed*, not written here.

It is not usable yet. `sparc#1103` reports AWS Labs definitions importing as a document
with **zero controls**, which leaves the converter nothing to map. So inherited rows
below carry `pending sparc#1103` rather than a control id: the capability exists, the
data path into it does not work today, and that is a different statement from either a
claim or a gap.

## Inline control comments

A control claim in the mapping must be checkable against the code that makes it. The
comment marks the site; the mapping is the index.

```go
// NIST: AC-3, AC-6
// Authorization is evaluated against the node the request names, not the endpoint it
// reaches. A handler that checks the endpoint passes for a node the caller may not read.
func (s *Server) authorizeNode(ctx context.Context, nodeUUID string) error {
```

```ts
// NIST: SI-10
// Overlay input is validated before it reaches the projection functions.
```

Rules:

- The tag is `NIST:` followed by comma-separated rev 5 control ids, enhancements in
  parentheses — `AU-9, SI-7(1)`.
- It goes on the declaration that implements the control, not on the file.
- Every id in a comment appears in the mapping, and every `implemented` row in the
  mapping names at least one location. The two are checked against each other; a claim
  with no site and a site with no claim are both defects.
- Comments state *why this satisfies the control*, not what the function does.

## OSCAL identifiers

Component and resource UUIDs are **deterministic UUIDv5** over a natural key, so
regenerating a definition does not churn identifiers and a federated peer can
deduplicate without coordinating — the same rule the product applies to its own
documents (`docs/03-data-model.md`).

```
namespace = uuidv5(URL_NAMESPACE, "https://risk-sentinel.org/ns/sparc")
          = d051648c-1ae1-569e-8569-b679a9aaf142
component  = uuidv5(namespace, "component:horizon-ledger")
resource   = uuidv5(namespace, "resource:nist-sp800-53-rev5-catalog")
```

Document-level UUIDs are the exception: they change per revision, because a revision is
a different document.

Custom properties are **not** invented here. The nine props under
`https://risk-sentinel.org/ns/sparc` are enumerated and constrained by
`schemas/sparc-namespace-props.v1.schema.json`, and v1 changes are additive only — a new
prop name means extending both the `enum` and the matching `allOf` branch. Where OSCAL
already has somewhere to put something, it goes there: `links` for references,
`remarks` for configuration dependencies, `implementation-status` for status.

## Updating this directory

`issue_rules.md` step 9 applies to any issue touching authentication, authorization,
audit, session management, crypto, signing, input validation, or configuration:

1. Update the affected rows in `nist-sp800-53-rev5-mapping.md` — status, summary, and
   the code location.
2. Update or add the component definition under `oscal/cdefs/`, with configuration
   dependencies in `remarks`.
3. Add or amend the inline control comments in the source touched.

A row whose evidence has not been produced yet stays `planned`. Moving a row to
`implemented` is a claim, and the reviewer is entitled to ask for the artifact.

## Validating the OSCAL

```bash
# The schema ships as a release asset, not in the OSCAL source tree —
# raw.githubusercontent paths under the tag return 404.
curl -sL -o /tmp/oscal_component_schema.json \
  https://github.com/usnistgov/OSCAL/releases/download/v1.2.1/oscal_component_schema.json

# draft-07, and ajv-formats must be loaded explicitly: without -c the schema
# itself is rejected for an "unknown format uri-reference" before it reads a
# single document, which looks like a schema error rather than a missing plugin.
npx ajv-cli@5 validate --spec=draft7 -c ajv-formats \
  -s /tmp/oscal_component_schema.json \
  -d 'docs/compliance/oscal/cdefs/*.json'
```

Verified against `v1.2.1` on 2026-09-19: `sparc-horizon.oscal.json valid`.

Tooling is not vendored in this repository; install on demand, as with the contract
checks in `CLAUDE.md`.
