# 09 Acceptance

## Criteria

- Fixtures for one federation, four organizations, seven boundaries, and about twenty systems pass `sparc-validate`, and UUIDs are identical across two regenerations.
- Every heatmap cell at every tier recomputes identically from exported OSCAL alone.
- The heat endpoint answers in under 300 ms at p95 with 10,000 controls.
- An ISO finds the top ATO blocker for their boundary in under 30 seconds in a moderated session.
- An attestation made in the UI reaches SPARC, is applied to the next HDF run through `saf attest`, and turns the cell green at every tier.
- An AO condition breach reopens the decision within one projection cycle.
- WCAG 2.2 AA: keyboard reachable, blockers carry a glyph as well as color, and both themes pass contrast checks.

## Pilot demo script

Rehearse it with [`demo/hud.html`](../demo/hud.html) before the real system exists.

1. The AO opens Enterprise IT and snaps to the nearest decision date; boundaries with blockers show red.
2. They drill to Portal, then the hottest cell, then a system, and read a control's evidence chain.
3. The ISO simulates the attestation to show the effect, then performs it for real with evidence.
4. The AO watches Portal clear at the organization tier and accepts a residual risk with a 14-day condition.
5. Moving the look-ahead past that condition reopens it on the AO's HUD.
