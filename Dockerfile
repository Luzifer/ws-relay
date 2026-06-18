FROM golang:1.26.4-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS builder

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
