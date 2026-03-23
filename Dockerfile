# syntax=docker/dockerfile:1.7-labs
# This Dockerfile requires BuildKit (for example via `docker buildx build`).
# It depends on BuildKit automatic platform arguments (`BUILDPLATFORM`,
# `TARGETOS`, `TARGETARCH`, and `TARGETVARIANT`). The legacy builder is
# unsupported.
# We use cross-compilation because QEMU is slow.
FROM --platform=${BUILDPLATFORM} golang:1.26.1-alpine3.22@sha256:07e91d24f6330432729082bb580983181809e0a48f0f38ecde26868d4568c6ac AS build

ARG GIT_DESCRIBE
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

# See .dockerignore for the list of files being copied.
WORKDIR "/src/"
# Add a download step to leverage Docker layer caching
COPY ["go.mod", "go.sum", "/src/"]
RUN go mod download

COPY --exclude=go.mod --exclude=go.sum [".", "/src/"]

# Compile the code.
RUN \
  CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
  go build -tags "timetzdata" -trimpath -ldflags="-w -s -X main.Version=${GIT_DESCRIBE} -buildid=" \
  -o /bin/ddns ./cmd/ddns

# The "alpine" stage can be used directly for debugging network issues.
FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS alpine
COPY --from=build /bin/ddns /bin/
USER 1000:1000
ENTRYPOINT ["/bin/ddns"]

# The minimal images contain only the program and the consolidated certificates.
FROM scratch AS minimal
COPY --from=build /bin/ddns /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 1000:1000
ENTRYPOINT ["/bin/ddns"]
