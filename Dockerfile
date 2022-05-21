FROM golang:alpine as builder

COPY . /go/src/github.com/Luzifer/ws-relay
WORKDIR /go/src/github.com/Luzifer/ws-relay

RUN set -ex \
 && apk add --update \
      build-base \
      git \
 && go install \
      -ldflags "-X main.version=$(git describe --tags --always || echo dev)" \
      -mod=readonly \
      -modcacherw \
      -trimpath


FROM alpine:latest

LABEL maintainer "Knut Ahlers <knut@ahlers.me>"

RUN set -ex \
 && apk --no-cache add \
      ca-certificates

COPY --from=builder /go/bin/ws-relay /usr/local/bin/ws-relay

EXPOSE 3000

ENTRYPOINT ["/usr/local/bin/ws-relay"]
CMD ["--"]

# vim: set ft=Dockerfile:
