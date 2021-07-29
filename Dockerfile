FROM --platform=linux/amd64 golang:alpine AS builder
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

FROM scratch
COPY --from=builder /bin/ddns /bin/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/ddns"]
