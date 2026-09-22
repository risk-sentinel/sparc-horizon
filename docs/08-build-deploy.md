# 08 Build and deploy

## Container

```dockerfile
FROM node:22 AS web
WORKDIR /web
COPY web/ .
RUN npm ci && npm run build

FROM golang:1.25 AS build
WORKDIR /src
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /horizon ./cmd/horizon

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /horizon /horizon
ENTRYPOINT ["/horizon", "serve"]
```

Pin base image digests in the real repo.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `HORIZON_SPARC_URL` | none | SPARC Delivery API |
| `HORIZON_SPARC_CA` | none | CA bundle for mTLS |
| `HORIZON_OIDC_ISSUER` | none | Agency IdP issuer URL |
| `HORIZON_DB_DSN` | `sqlite:///data/horizon.db` | Ledger and projections |
| `HORIZON_EVIDENCE_URI` | none | Object store for evidence |
| `HORIZON_SIGNING_CERT` | none | PKI certificate and key reference |
| `HORIZON_BUCKETS` | `0,7,14,30` | Projection horizons in days |
| `HORIZON_NAMESPACE` | `https://sparc.risk-sentinel.org/ns` | Namespace contract |

## Terraform

```hcl
module "horizon" {
  source          = "./deploy/terraform/horizon"
  image           = "registry.example.gov/horizon:0.1.0"
  sparc_url       = var.sparc_url
  oidc_issuer     = var.oidc_issuer
  evidence_bucket = aws_s3_bucket.evidence.id
}
```

A Helm chart under `deploy/helm/` covers Kubernetes targets with the same variables.
