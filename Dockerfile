# syntax=docker/dockerfile:1
# SPARC Horizon: API v0 over the fixture federation, the HUD at /, and the
# static design demos at /demo/.
# Runs cmd/mockserver until cmd/horizon exists (docs/08-build-deploy.md).
#
# Base images are pinned by digest only (Sonar docker:S6596). Bump deliberately
# and re-measure with the scanners.
#   golang 1.27.4    (matches ci.yml's go-version line)
#   ubi10/ubi 10.3   (builder for the runtime root filesystem, never shipped)
#   ubi10/nodejs-22-minimal 10.3-1793848662, Node 22.23.2 (builds web/, never shipped)
ARG NODE_IMAGE=registry.access.redhat.com/ubi10/nodejs-22-minimal@sha256:ea24e8ec875b6353d2d3942214a01dad62528a37823e92094a9019a5c058ee5f
ARG GO_IMAGE=golang@sha256:fe517acaa580cc7915457080f850c37e618b3c1e750f98b58fc6429b605147a1
ARG UBI_BUILDER=registry.access.redhat.com/ubi10/ubi@sha256:73ea54bfbfb586075bd3d45df6483a7f68875233c8cc7cebcf9b2bb42306c308

# The HUD (#122): static files, so built once on the build platform. The lockfile
# is the whole dependency set and install scripts never run (web/.npmrc).
FROM --platform=$BUILDPLATFORM ${NODE_IMAGE} AS web
WORKDIR /opt/app-root/src
COPY --chown=1001:0 web/package.json web/package-lock.json web/.npmrc ./
RUN npm ci --ignore-scripts --no-audit --no-fund
COPY --chown=1001:0 web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
# Static binary: nothing to link against at runtime (CM-7, SI-7).
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/horizon ./cmd/mockserver
# The served tree, read-only for every user whatever modes the context carries.
COPY fixtures/ /out/srv/fixtures/
COPY demo/ /out/srv/demo/
COPY --from=web /opt/app-root/src/dist/ /out/srv/web/
RUN chmod -R u=rwX,go=rX /out/srv

# The runtime root filesystem, built from Red Hat's signed UBI 10 packages and
# nothing else: no shell, no utilities, no libraries (CM-7). Four packages, so the
# rpm database lists exactly what is in the image. The CA bundle is copied as a
# file: the ca-certificates package would pull in bash and coreutils. The content
# sets name the repositories the packages came from (BaseOS, target arch only),
# which is how scanners map them to Red Hat advisories.
FROM --platform=$BUILDPLATFORM ${UBI_BUILDER} AS rootfs
ARG TARGETARCH
RUN case "${TARGETARCH}" in \
      amd64) arch=x86_64 ;; \
      arm64) arch=aarch64 ;; \
      *) echo "unsupported TARGETARCH ${TARGETARCH}" >&2; exit 1 ;; \
    esac \
    && dnf --installroot=/rootfs --releasever=10 --forcearch="${arch}" \
      --setopt=install_weak_deps=False --setopt=tsflags=nodocs,noscripts -y \
      install filesystem-3.18-17.el10 redhat-release-10.3-15.el10 setup-2.14.5-7.el10 tzdata-2026f-1.el10_3 \
    && dnf --installroot=/rootfs --releasever=10 clean all \
    && rm -rf /rootfs/var/cache/dnf /rootfs/var/lib/dnf /rootfs/var/log/dnf* /rootfs/var/log/hawkey.log \
    && mkdir -p /rootfs/etc/pki/tls/certs /rootfs/usr/share/buildinfo /rootfs/root/buildinfo \
    && cp /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem /rootfs/etc/pki/tls/certs/ca-bundle.crt \
    && python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); \
d["content_sets"]=[c for c in d["content_sets"] if "-for-" + sys.argv[3] + "-baseos-" in c]; \
assert len(d["content_sets"]) == 2, d["content_sets"]; \
json.dump(d, open(sys.argv[2], "w"), indent=2)' \
      /usr/share/buildinfo/content-sets.json /rootfs/usr/share/buildinfo/content-sets.json "${arch}" \
    && cp /rootfs/usr/share/buildinfo/content-sets.json /rootfs/root/buildinfo/content-sets.json

FROM scratch
COPY --from=rootfs /rootfs/ /
COPY --from=build /out/horizon /usr/local/bin/horizon
COPY --from=build /out/srv/ /srv/horizon/
# Non-root, and the served tree is read-only to the process (AC-6).
USER 1001:1001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/horizon"]
# The fixtures' horizon, so docs/testing.md's outputs reproduce. Point -dir at
# your own OSCAL to see your own boundary.
CMD ["-addr", "0.0.0.0:8080", "-dir", "/srv/horizon/fixtures", "-static", "/srv/horizon/web", "-demo", "/srv/horizon/demo", "-horizon", "2026-10-15"]
