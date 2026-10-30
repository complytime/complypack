FROM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o complypack ./cmd/complypack

FROM registry.access.redhat.com/ubi9-micro:9.8@sha256:5be877a2a280ae36eb9ab80ca4efb28bbf77c57abbba38137cdc37a03b8465e6

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt
COPY --from=builder /build/complypack /usr/local/bin/complypack

ENV DOCKER_CONFIG=/.docker
ENV XDG_CACHE_HOME=/tmp/cache

LABEL io.modelcontextprotocol.server.name="io.github.complytime/complypack"

ARG USER_UID=10001
USER ${USER_UID}

ENTRYPOINT ["complypack"]
