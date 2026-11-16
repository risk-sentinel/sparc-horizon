# 08 Build and deploy

## Container

```dockerfile
FROM node:22 AS web
WORKDIR /web
COPY web/ .
RUN npm ci && npm run build

FROM golang:1.27 AS build
WORKDIR /src
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /horizon ./cmd/horizon

FROM registry.access.redhat.com/ubi10/ubi AS rootfs
RUN dnf --installroot=/rootfs --releasever=10 --setopt=install_weak_deps=False \
      --setopt=tsflags=nodocs,noscripts -y install filesystem redhat-release setup tzdata \
    && dnf --installroot=/rootfs --releasever=10 clean all
# plus the CA bundle and the BaseOS content sets, copied as files

FROM scratch
COPY --from=rootfs /rootfs/ /
COPY --from=build /horizon /usr/local/bin/horizon
USER 1001:1001
ENTRYPOINT ["/usr/local/bin/horizon", "serve"]
```

The root filesystem is built from Red Hat's signed UBI 10 packages, the same lineage as `sparc`'s image, and
holds only four of them: `filesystem`, `redhat-release`, `setup` and `tzdata`. There is no shell, no utility
and no shared library, so the service is the only program in the image, and it runs as uid 1001. The rpm
database lists exactly those four packages, so a scanner reports only what is present. Two files are copied
in rather than installed:

- the CA bundle, because the `ca-certificates` package would pull in `bash`, `coreutils` and 35 more;
- the BaseOS content sets (`/usr/share/buildinfo/content-sets.json`), which scanners use to map the packages to
  Red Hat advisories. Without it Trivy fails with "unable to find CPE indices".

The repository's `Dockerfile` builds this shape today (#108), with the builders and package versions pinned. Until
`cmd/horizon` exists it builds `cmd/mockserver` instead, and copies in the fixture federation and the HUD
demos:

```bash
docker build -t sparc-horizon:dev .
docker run --rm -p 8080:8080 sparc-horizon:dev
# HUD demos at http://localhost:8080/ and API v0 at http://localhost:8080/v1/
# (persona header: X-Horizon-Persona, see fixtures/api/personas.json)
```

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
