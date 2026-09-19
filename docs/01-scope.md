# 01 Scope

## The claim

A person at the lowest level can see what will block an authorization before it does, and close it, while the AO watches the effect roll up.

## Division of responsibility

| SPARC | Horizon |
|---|---|
| Pipeline translation (HDF v3 through the HDF CLI) | Attestations for M&O controls |
| Framework crosswalks and mapping documents | AO decisions and conditions |
| Catalogs, baselines, and profile resolution | Forward projections of posture |
| Federation trust fabric (PKI, peer instances) | The HUD and role lenses |

## In scope

- A four-tier hierarchy (federation, organization, boundary, system) built from OSCAL party and SSP metadata
- Projected posture at any look-ahead date, with nominal families collapsed
- The M&O attestation lifecycle with signed, hashed evidence in back-matter
- AO risk acceptance with conditions that reopen on breach
- Round trip to the pipeline through `saf attest` files
- Axis swap between 800-53 families and FedRAMP 20x KSI themes through SPARC mapping documents

## Out of scope for the prototype

- Authoring catalogs, profiles, or mappings (stays in SPARC)
- Scanner conversions (the reusable GitLab include and HDF CLI handle them)
- Replacing the GRC tool; Horizon exports OSCAL that any GRC can render
- Multi-region high availability and production-scale Postgres tuning
- Briefing mode slides (planned after the pilot)

## HUD principles

- **Project forward.** Headlines describe posture on the decision date, not last scan's percentage.
- **One focal point.** Each lens leads with a single next best action.
- **Consequence with the item.** Every item says what happens if it is ignored.
- **Management by exception.** Green is quiet; nominal families collapse.
- **Countdowns over timestamps.** Days until breach, not "last scanned on".
- **Act in place.** Attest, attach evidence, request a waiver, or escalate from the card.
- **One grammar at every tier.** Rows are the children of the current node; columns are families.
