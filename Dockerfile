# We use cross-compilation because QEMU is slow. linux/amd64 is what GitHub uses.
FROM --platform=linux/amd64 golang:1.19.3-alpine3.16@sha256:3607071679bd7702e5461ff72ad0886760a03cb00a09163be3020d8f1cda5299 AS build
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
  go build -tags timetzdata -trimpath -ldflags="-w -s -X main.Version=${GIT_DESCRIBE}" \
  -o /bin/ddns cmd/ddns/*

# The minimal images contain only the program and the consolidated certificates.
FROM scratch AS minimal
COPY --from=build /bin/ddns /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/ddns"]
