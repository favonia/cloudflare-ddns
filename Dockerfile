# syntax=docker/dockerfile:1.7-labs
# We use cross-compilation because QEMU is slow.
FROM --platform=${BUILDPLATFORM} golang:1.23.4-alpine3.20@sha256:9a31ef0803e6afdf564edc8ba4b4e17caed22a0b1ecd2c55e3c8fdd8d8f68f98 AS build

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
FROM alpine AS alpine
RUN apk add --no-cache ca-certificates-bundle
COPY --from=build /bin/ddns /bin/
USER 1000:1000
ENTRYPOINT ["/bin/ddns"]

# The minimal images contain only the program and the consolidated certificates.
FROM scratch AS minimal
COPY --from=build /bin/ddns /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 1000:1000
ENTRYPOINT ["/bin/ddns"]
