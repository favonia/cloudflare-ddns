FROM --platform=linux/amd64 golang:alpine AS builder
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN \
  apk update && \
  apk add --no-cache git ca-certificates && \
  update-ca-certificates
WORKDIR $GOPATH/src/github.com/favonia/cloudflare-ddns-go/
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build -tags timetzdata -o /bin/ddns -ldflags="-w -s" cmd/*.go

FROM scratch
COPY --from=builder /bin/ddns /bin/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/ddns"]
