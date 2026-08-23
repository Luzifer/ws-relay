FROM golang:1.27.0-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS builder

COPY . /go/src/ws-relay
WORKDIR /go/src/ws-relay

ENV CGO_ENABLED=0

RUN <<-EOF
  set -ex

  apk add --update \
    build-base \
    git

  install -dm0755 /rootfs/usr/bin

  go build \
    -ldflags "-X main.version=$(git describe --tags --always || echo dev)" \
    -mod=readonly \
    -modcacherw \
    -trimpath \
    -o /rootfs/usr/bin/ws-relay
EOF


FROM scratch

LABEL org.opencontainers.image.authors="Knut Ahlers" \
      org.opencontainers.image.source="https://github.com/Luzifer/ws-relay" \
      org.opencontainers.image.title="ws-relay"

COPY --from=builder /rootfs/ /

EXPOSE 3000
ENTRYPOINT ["/usr/bin/ws-relay"]
