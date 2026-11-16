# syntax=docker/dockerfile:1
# SPARC Horizon: API v0 over the fixture federation, plus the HUD demos.
# Runs cmd/mockserver until cmd/horizon exists (docs/08-build-deploy.md).
#
# Base images are pinned by digest only (Sonar docker:S6596). Bump deliberately
# and re-measure with the scanners.
#   golang 1.27.4                  (matches ci.yml's go-version line)
#   ubi10/ubi-micro 10.3-1794483856 (no package manager; shell stripped below)
ARG GO_IMAGE=golang@sha256:fe517acaa580cc7915457080f850c37e618b3c1e750f98b58fc6429b605147a1
ARG UBI_IMAGE=registry.access.redhat.com/ubi10/ubi-micro@sha256:2ad0f98cb13160c4773c945ef93f88b25831c7f27cd3780d72a7e78b210ea72b

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
# Static binary: nothing to link against in ubi-micro (CM-7, SI-7).
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/horizon ./cmd/mockserver

FROM ${UBI_IMAGE} AS ubi

# ubi-micro without its executables: no shell or utilities at runtime, so the
# server is the only program in the image (CM-7). Copied rather than deleted, so
# the shell is never in any layer. The rpm database and release files come with
# it, so scanners still see a UBI 10 image. The bin/sbin links and the debug
# build-id links point into the excluded directories, so they go too.
FROM scratch
COPY --from=ubi \
    --exclude=bin --exclude=sbin \
    --exclude=usr/bin --exclude=usr/sbin --exclude=usr/libexec \
    --exclude=usr/lib/.build-id --exclude=usr/lib/debug \
    / /
COPY --from=build /out/horizon /usr/local/bin/horizon
COPY fixtures/ /srv/horizon/fixtures/
COPY demo/ /srv/horizon/demo/
# Non-root, and the served tree is read-only to the process (AC-6).
USER 1001:1001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/horizon"]
CMD ["-addr", "0.0.0.0:8080", "-dir", "/srv/horizon/fixtures", "-static", "/srv/horizon/demo"]
