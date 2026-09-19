# 07 Security

| Area | Approach |
|---|---|
| Identity | OIDC with the agency IdP. Party UUIDs map to subjects, so roles come from `responsible-parties` in the documents |
| Authorization | Roles bind to a node and inherit downward. Every API handler checks the node, not the endpoint |
| Integrity | The ledger is hash-chained; evidence is hashed and signed; exports carry the chain head for verification |
| Federation | mTLS to SPARC peers using the existing trust fabric; bundles are verified before ingestion |
| What-if isolation | Overlays are copy-on-write, never persisted, and cannot emit OSCAL or `saf attest` files |
| Container | Distroless, non-root, static binary; SBOM and image signature produced in the pipeline |
| Dogfooding | Horizon runs through the same reusable SAF pipeline and produces its own HDF and OSCAL package |
