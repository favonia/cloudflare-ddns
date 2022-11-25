# The Go compiler is run under linux/amd64 because GitHub Actions is using linux/amd64
# and it is slow to run the compiler via QEMU.
FROM --platform=linux/amd64 golang:1.19.3-alpine3.16@sha256:d171aa333fb386089206252503bc6ab545072670e0286e3d1bbc644362825c6e AS build
ARG GIT_DESCRIBE
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
WORKDIR "/src/"
COPY ["go.mod", "go.mod"]
COPY ["go.sum", "go.sum"]
COPY ["internal", "internal"]
COPY ["cmd", "cmd"]
RUN \
  CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
  go build -tags timetzdata -ldflags="-w -s -X main.Version=${GIT_DESCRIBE}" \
  -o /bin/ddns cmd/ddns/*

# The minimal images contain only the program and the consolidated certificates.
FROM scratch AS minimal
COPY --from=build /bin/ddns /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/ddns"]
