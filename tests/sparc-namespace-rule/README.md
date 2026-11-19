# SPARC's namespace-props rule, over Horizon's fixtures

P0's exit criterion "fixtures pass `sparc-validate`" asked SPARC for rules that refuse a document
whose SPARC-namespace props are wrong (`sparc#1154`). SPARC delivered them as
`SparcNamespacePropsRule`, an export gate in `sparc`, not in `sparc-validate`. It checks four things
the namespace schema alone cannot: each prop's value against the schema, its **placement** (which
object may carry it), **uniqueness** per props array, and **presence** of the props an SSP must carry.

`harness.rb` loads that rule from a read-only `sparc` checkout and runs it over a folder of OSCAL.
It stubs only what Rails would supply: `Rails.root`, the error class, and `SparcNamespaceProps`'
namespace URI, name list and `ours?`, copied verbatim. The rule itself is not copied, so a change
upstream is what runs.

```bash
ruby tests/sparc-namespace-rule/harness.rb ../sparc fixtures/oscal
```

Measured 2026-11-18 (#122) against `sparc` at `44585a6d`, Ruby 3.4.9, json_schemer 2.5.0:

| Input | Result |
|---|---|
| `fixtures/oscal/` | 21 documents checked (7 SSP, 7 assessment results, 7 POA&M), **0 failed**, 150 SPARC-namespace props |
| A copy with `blocks-ato: "maybe"` on a risk, and `blocks-ato` on SSP metadata | 2 checked, **2 failed**: `value ... is not one of: ["true", "false"]` and `belongs on a risk, not here` |

**Not exercised:** the presence check. It takes the props an SSP must carry from SPARC's own record
of the system (`required_metadata`), which a fixture has no equivalent of, so the harness passes
none. The schema files are byte-identical in both repositories (SPARC's provenance sidecar records
the commit it vendored).

Not in CI: it needs a `sparc` checkout and a Ruby toolchain, and the estate rule is that this
repository never writes to a sibling. Re-run it when the fixtures or the namespace schema change.
