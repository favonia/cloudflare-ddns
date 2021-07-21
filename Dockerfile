FROM --platform=linux/amd64 golang:alpine AS builder
ARG GIT_DESCRIBE
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN \
  apk update && \
  apk add --no-cache git ca-certificates && \
  update-ca-certificates
WORKDIR $GOPATH/src/github.com/favonia/cloudflare-ddns-go/
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build -tags timetzdata -o /bin/ddns -ldflags="-w -s" -ldflags="-X main.version=${GIT_DESCRIBE}" cmd/*.go

FROM scratch
COPY --from=builder /bin/ddns /bin/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/ddns"]
