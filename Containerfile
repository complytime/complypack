FROM golang:1.26-alpine@sha256:0648ddfa35769070197ba1cdf22a16dc452caf9315e66b91791308a543baf229 AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o complypack ./cmd/complypack

FROM registry.access.redhat.com/ubi9-micro:9.8@sha256:35de56a9413112f1474e392ebc35e0cf6f0fb484c8e8877bbae59b513694b41f

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt
COPY --from=builder /build/complypack /usr/local/bin/complypack

ENV DOCKER_CONFIG=/.docker
ENV XDG_CACHE_HOME=/tmp/cache

LABEL io.modelcontextprotocol.server.name="io.github.complytime/complypack"

ARG USER_UID=10001
USER ${USER_UID}

ENTRYPOINT ["complypack"]
