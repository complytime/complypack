FROM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o complypack ./cmd/complypack

FROM registry.access.redhat.com/ubi9-micro:9.8@sha256:7a0454cbd9bd847e8f6a63b6f0254a6efbeb6e0ed71a5d824a4f6cccbe626650

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt
COPY --from=builder /build/complypack /usr/local/bin/complypack

ENV DOCKER_CONFIG=/.docker
ENV XDG_CACHE_HOME=/tmp/cache

LABEL io.modelcontextprotocol.server.name="io.github.complytime/complypack"

ARG USER_UID=10001
USER ${USER_UID}

ENTRYPOINT ["complypack"]
