# The Go compiler is run under linux/amd64 because GitHub Actions is using linux/amd64
# and it is slow to run the compiler via QEMU.
FROM --platform=linux/amd64 golang:alpine AS build
ARG GIT_DESCRIBE
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN \
  apk update && \
  apk add --no-cache git ca-certificates && \
  update-ca-certificates
WORKDIR "/src/"
COPY ["go.mod", "go.mod"]
COPY ["go.sum", "go.sum"]
COPY ["internal", "internal"]
COPY ["cmd", "cmd"]
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build -tags timetzdata -o /bin/ddns -ldflags="-w -s -X main.Version=${GIT_DESCRIBE}" cmd/*.go

# After the compilation is done, we copied the program into alpine images
# with matching architectures.
FROM alpine AS alpine
RUN \
  apk update && \
  apk add --no-cache ca-certificates && \
  update-ca-certificates
COPY --from=build /bin/ddns /bin/
ENTRYPOINT ["/bin/ddns"]

# The minimal images contain only the program and the consolidated certificates.
FROM scratch AS minimal
COPY --from=build /bin/ddns /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/ddns"]
