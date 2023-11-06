# We use cross-compilation because QEMU is slow.
FROM --platform=${BUILDPLATFORM} golang:1.21.3-alpine3.18@sha256:96a8a701943e7eabd81ebd0963540ad660e29c3b2dc7fb9d7e06af34409e9ba6 AS build
ARG GIT_DESCRIBE
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

# See .dockerignore for the list of files being copied.
WORKDIR "/src/"
COPY [".", "/src/"]

# Compile the code.
RUN \
  CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
  go build -tags timetzdata -trimpath -ldflags="-w -s -X main.Version=${GIT_DESCRIBE} -buildid=" \
  -o /bin/ddns ./cmd/ddns

# The minimal images contain only the program and the consolidated certificates.
FROM scratch AS minimal
COPY --from=build /bin/ddns /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/ddns"]
