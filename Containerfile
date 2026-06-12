FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o complypack ./cmd/complypack

FROM registry.access.redhat.com/ubi9-micro:9.6-4@sha256:b498b3ea26111ab4b81d65139f2ebd2ef9a2abb7a4588b7fdcc54889f95e9caa

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt
COPY --from=builder /build/complypack /usr/local/bin/complypack

ARG USER_UID=10001
USER ${USER_UID}

ENTRYPOINT ["complypack"]
