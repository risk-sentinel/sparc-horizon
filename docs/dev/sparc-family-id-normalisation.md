# `key-grammar.v1.json` lowercases `family-id` outside its vocabulary

**Status: filed 2026-09-22 as [`sparc#1175`](https://github.com/risk-sentinel/sparc/issues/1175).**
A disagreement with a sibling repository's contract (`risk-sentinel/sparc`,
`lib/federation/key-grammar.v1.json`). This is the source text; the issue carries everything
below the line. It stays here because the test that pins the divergence lives here too — if the
disagreement is settled in SPARC's favour, Horizon adopts the rule and both go.

Found while adopting the registered namespace and reconciling Horizon's Go reference with the
shared contract (#54). The consequence — that two implementations derive different identifiers
for one projection cell — is recorded in [`docs/03-data-model.md`](../03-data-model.md) and
pinned by a test in `internal/keys`, so it cannot quietly resolve or widen.

---

## Summary

The shared contract normalises `family-id` with `lowercase`, unconditionally, in the `cell`
field list:

```json
{ "one-of": ["control-id", "family-id"],
  "normalise": { "control-id": "by-vocabulary", "family-id": "lowercase" } }
```

`control-id` dispatches on the vocabulary. `family-id` does not. That reintroduces, for family
identifiers, the defect that scoping removed from control identifiers in `sparc-horizon#37` —
which this contract otherwise implements faithfully.

**An AWS Security Hub family is `ACM`. Lowercased it is `acm`, which names nothing in any
catalog.** The same argument the contract already accepts for `ACM.1` applies unchanged one
level up.

## Why it is not only a tidiness question

Horizon holds the vocabulary-scoped rule, so the two implementations **derive different
identifiers for the same object**, and neither errors:

| | `family-id` hashed | Cell identifier |
|---|---|---|
| Shared contract | `acm` | `f2a38fbf-bed3-5e41-b95e-c6b8c88a0c00` |
| Horizon (`internal/keys`) | `ACM` | `b9691843-8d7e-5b3e-a58a-4a56414e4dba` |

Both derived under the registered namespace `9f434272-f796-589b-b972-954790395630`, so the
difference is the normalisation and nothing else.

Under `node-uuid` `2b0b4a6c-7d31-4e08-95af-6c1e8b204d7a`, `source-uuid`
`5d40a72c-3e18-4f9b-86d2-0c7a41b5e926`, vocabulary `opaque`, bucket `today`.

Two identifiers for one cell, no error on either side, is the silent divergence the shared
vectors exist to prevent. **No vector catches it**, because every vector that exercises a family
uses a NIST one, where both rules agree.

## The rule this contradicts

`sparc-horizon#37` scoped canonicalisation to a vocabulary and SPARC accepted the reasoning when
it adopted the field lists. The contract states it plainly under `control-id`:

> Under `nist-sp800-53` the CANONICALISED value must match `^[a-z]{2,3}-\d+(\.\d+)*$` — which
> rejects […] a foreign-vocabulary identifier (`ACM.1` canonicalises to `acm.1`, which validates
> and names nothing).

A normalisation that produces a value which "validates and names nothing" is worse than a
rejection, because nothing downstream can tell. That is as true of `ACM` as of `ACM.1`.

## Suggested fix

Make `family-id` dispatch the way `control-id` already does:

```json
"normalise": { "control-id": "by-vocabulary", "family-id": "by-vocabulary" }
```

with the existing `vocabulary-normalisers` mapping — `nist-sp800-53` → lowercase,
`opaque` → none. The machinery is present; `family-id` simply is not routed through it.

The `family-id` type rule then reads "`^[a-z]{2,3}$` under `nist-sp800-53`; any non-empty value
under an opaque vocabulary", matching how `control-id` is already specified.

**A vector would make the fix self-checking**: a projection cell keyed on a foreign-vocabulary
family, which is the case the current vector set does not cover.

## A second, smaller point

`decision-date` is typed `^\d{4}-\d{2}-\d{2}$` and nothing more, so `2026-13-01` and
`2026-02-31` are accepted as the day an authorizing official decided something. Horizon matches
the contract exactly rather than being stricter — refusing an identifier a peer has already
derived is the failure this grammar exists to prevent — so both implementations currently key
decisions on impossible dates.

Not urgent and not a divergence, since both sides agree. Worth a calendar check in the rule if
the contract ever tightens.

## Reproducer

Against Horizon's Go reference, which is the implementation holding the other rule:

```go
d := keys.New(keys.Namespace())
src := keys.Source{UUID: "5d40a72c-3e18-4f9b-86d2-0c7a41b5e926", Vocabulary: canonical.VocabOpaque}
node := "2b0b4a6c-7d31-4e08-95af-6c1e8b204d7a"

upper, _ := d.CellForFamily(node, src, "ACM", keys.BucketToday)  // b9691843-8d7e-5b3e-a58a-4a56414e4dba
lower, _ := d.CellForFamily(node, src, "acm", keys.BucketToday)  // f2a38fbf-bed3-5e41-b95e-c6b8c88a0c00
// Under the shared contract both inputs derive the second value.
```

It runs as `TestFamilyIDNormalisationDivergesFromTheContract` in `internal/keys`, which asserts
the divergence rather than tolerating it: if the two ever derive one identifier, the test fails
and this document is stale.

## Environment

| | |
|---|---|
| Contract | `risk-sentinel/sparc`, `lib/federation/key-grammar.v1.json`, commit `33a25b3bf76ea1960f1fa1a7a372108fd64fee28` |
| Vendored here | `internal/keys/testdata/`, with `PROVENANCE.json` |
| Horizon | `internal/keys`, `internal/canonical`, at the registered namespace `9f434272-f796-589b-b972-954790395630` |
| Everything else | Conforms: all 26 vectors, the nine field lists, and every other type rule |
