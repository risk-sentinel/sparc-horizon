# 05 Projection engine

Everything in a cell comes from OSCAL fields, so any cell can be recomputed from the exported documents alone. That recomputation is the audit test.

```go
func StateAt(c Control, t time.Time) State {
    down := c.Failing ||
        (c.Expires != nil && !c.Expires.After(t)) ||
        (c.MilestoneOpen && c.Milestone != nil && !c.Milestone.After(t))
    return State{Down: down, Blocks: down && c.BlocksATO, Inherited: c.Inherited}
}

func Roll(kids []Agg) (a Agg) {
    for _, k := range kids {
        a.Total += k.Total * k.Weight // FIPS 199 weight
        a.Pass += k.Pass * k.Weight
        a.Blockers += k.Blockers      // any blocker stays visible upward
        a.Worsening = a.Worsening || k.Worsening
    }
    return a
}
```

## Cell rules

| Channel | Source |
|---|---|
| Fill | Projected pass ratio at the horizon: nominal ≥ 95%, watch ≥ 85%, degraded below that |
| Blocks ATO (red, with `!` glyph) | Any open risk with `blocks-ato` |
| Outline | An `expires` date or POA&M milestone falls inside the look-ahead window |
| Inherited | Implemented through an `inherited` statement |
| Column order | Blockers descending, then pass ratio ascending; nominal columns collapse into `+N` |
| Column axis | Swapped using SPARC mapping documents (800-53 families or KSI themes) |

## Next-best-action ranking

```text
score = ATO impact × urgency × crosswalk reach ÷ effort
urgency = 1 ÷ days until breach (capped)
reach   = framework requirements cleared, from SPARC mappings
```

Weights are configuration, and every ranking is logged so the ordering can be reviewed with users.

## Materialization

- Projections are materialized per node for horizon buckets: today, +7, +14, +30, and every decision date.
- Buckets are invalidated along the node's ancestry whenever a ledger event lands.
- Arbitrary slider values are computed on demand from the nearest bucket.
- `horizon rebuild` recomputes everything from the ledger.
- What-if overlays reuse the same functions over a copy-on-write view of the ledger.
