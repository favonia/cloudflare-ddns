# syntax=docker/dockerfile:1.7-labs
# This Dockerfile requires BuildKit (for example via `docker buildx build`).
# It depends on BuildKit automatic platform arguments (`BUILDPLATFORM`,
# `TARGETOS`, `TARGETARCH`, and `TARGETVARIANT`). The legacy builder is
# unsupported.
# We use cross-compilation because QEMU is slow.
FROM --platform=${BUILDPLATFORM} golang:1.26.3-alpine3.22@sha256:be93003ee861b3b91b6ebcb22678524947e0cd786c2df3f32af520006b1e54f5 AS build

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
FROM alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS alpine
COPY --from=build /bin/ddns /bin/
USER 1000:1000
ENTRYPOINT ["/bin/ddns"]

# The minimal images contain only the program and the consolidated certificates.
FROM scratch AS minimal
COPY --from=build /bin/ddns /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 1000:1000
ENTRYPOINT ["/bin/ddns"]
