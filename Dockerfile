FROM golang:1.27.1-alpine@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS builder

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
