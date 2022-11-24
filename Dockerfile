# Target-specific build arguments using https://github.com/docker/buildx/issues/157#issuecomment-539457548

FROM golang:1.19.3-alpine3.16@sha256:d171aa333fb386089206252503bc6ab545072670e0286e3d1bbc644362825c6e AS build-386
ENV USE_PIE=true

FROM golang:1.19.3-alpine3.16@sha256:d171aa333fb386089206252503bc6ab545072670e0286e3d1bbc644362825c6e AS build-amd64
ENV USE_PIE=true

FROM golang:1.19.3-alpine3.16@sha256:d171aa333fb386089206252503bc6ab545072670e0286e3d1bbc644362825c6e AS build-armv6
ENV USE_PIE=true

FROM golang:1.19.3-alpine3.16@sha256:d171aa333fb386089206252503bc6ab545072670e0286e3d1bbc644362825c6e AS build-armv7
ENV USE_PIE=true

FROM golang:1.19.3-alpine3.16@sha256:d171aa333fb386089206252503bc6ab545072670e0286e3d1bbc644362825c6e AS build-arm64
ENV USE_PIE=true

FROM golang:1.19.3-alpine3.16@sha256:d171aa333fb386089206252503bc6ab545072670e0286e3d1bbc644362825c6e AS build-ppc64le
ENV USE_PIE=true

FROM --platform=linux/amd64 golang:1.19.3-alpine3.16@sha256:d171aa333fb386089206252503bc6ab545072670e0286e3d1bbc644362825c6e AS build-riscv64
ENV USE_PIE=false

FROM golang:1.19.3-alpine3.16@sha256:d171aa333fb386089206252503bc6ab545072670e0286e3d1bbc644362825c6e AS build-s390x
ENV USE_PIE=true

ARG TARGETARCH
ARG TARGETVARIANT
FROM build-${TARGETARCH}${TARGETVARIANT} AS build
ARG GIT_DESCRIBE
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN \
  if [ "x${USE_PIE}" = "xtrue" ] ; then \
    apk update && apk add --no-cache gcc musl-dev; \
  fi
WORKDIR "/src/"
COPY ["go.mod", "go.mod"]
COPY ["go.sum", "go.sum"]
COPY ["internal", "internal"]
COPY ["cmd", "cmd"]
RUN \
  if [ "x${USE_PIE}" = "xtrue" ] ; then \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -tags timetzdata -ldflags "-w -s -X main.Version=${GIT_DESCRIBE} -linkmode=external -extldflags=--static-pie" -buildmode=pie \
    -o /bin/ddns cmd/*.go; \
  else \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -tags timetzdata -ldflags="-w -s -X main.Version=${GIT_DESCRIBE}" \
    -o /bin/ddns cmd/*.go; \
  fi

# The minimal images contain only the program and the consolidated certificates.
FROM scratch AS minimal
COPY --from=build /bin/ddns /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/ddns"]
