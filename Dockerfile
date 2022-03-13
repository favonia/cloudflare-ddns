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

FROM alpine AS alpine
RUN \
  apk update && \
  apk add --no-cache ca-certificates && \
  update-ca-certificates
COPY --from=build /bin/ddns /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/ddns"]

FROM scratch AS minimal
COPY --from=build /bin/ddns /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/bin/ddns"]
