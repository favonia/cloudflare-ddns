FROM --platform=linux/amd64 golang:alpine AS builder
ARG TARGETOS
ARG TARGETARCH
RUN \
  apk update && \
  apk add --no-cache git ca-certificates && \
  update-ca-certificates
WORKDIR $GOPATH/src/github.com/favonia/cloudflare-ddns-go/
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /go/bin/ddns -ldflags="-w -s" cmd/ddns.go

FROM scratch
COPY --from=builder /go/bin/ddns /go/bin/ddns
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/go/bin/ddns"]
