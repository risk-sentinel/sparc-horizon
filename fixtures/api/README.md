# API v0 golden responses

**Generated. Do not edit by hand.** Written by `internal/fixtures` alongside the OSCAL,
and rewritten wholesale by `go run ./cmd/genfixtures`.

Served by `go run ./cmd/mockserver`, which P3 builds the HUD against and P1 shapes its
client against. Every response validates against `api/openapi.yaml` in CI — both that each
golden satisfies the schema its path declares, and that the contract declares nothing the
mock cannot answer.

## These are illustrative shape, not projections

Cell states come from `project.StateAt` over the fixtures' own observation expiries and
blocking risks. That is the algorithm `docs/05-projection-engine.md` specifies, but this is
**not the projection engine**: there is no ledger, no materialisation, no invalidation along
a node's ancestry, and no ranking model. P2 builds those.

So: do not cite a number here as an engine result, and do not treat the ordering as the
ranking. What these are good for is shape, volume and the joins between endpoints.

Every value is derived by **parsing the OSCAL this same run emitted**, rather than from the
generator's own state. That is the cheap version of the audit claim — if a response cannot
be recomputed from exported documents, it does not belong in the contract either.

## Horizon

Computed once, for **2026-10-15**. The `horizon` query parameter is
accepted and ignored by the mock; a real service projects to the date it is asked for.

## Personas

Roles bind to a node and inherit downward, so each persona sees a **different tree** rather
than the same tree with parts greyed out. Select one with the `X-Horizon-Persona` header.

| Persona | Role | Sees |
|---|---|---|
| `ao-ods` | authorizing-official | Sees the organization, its boundaries and their systems. Roles inherit downward. |
| `so-ods-portal` | system-owner | Sees one boundary and its systems. Everything above it is 404, not 403. |
| `iso-bgm-grants` | information-system-security-officer | A different organization entirely, to show that two personas do not overlap. |

**A node outside the caller's tree returns 404, not 403** — identical to a node that does
not exist. The mock enforces that rather than describing it, because a client written
against a lenient mock is a client that breaks on the real service.

## The 800-53 axis

Heat and cell responses carry NIST families only. The fixtures also contain AWS Security Hub
controls, reaching a boundary through an inherited component definition — they are **off this
axis** until SPARC's crosswalk maps them (`sparc#1103`). Putting `ACM` beside `AC` would claim
a mapping this repository does not own.
