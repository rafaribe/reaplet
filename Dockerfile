# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26

# Build natively on the runner's architecture, cross-compile via GOARCH.
# This avoids QEMU emulation which makes modernc.org/sqlite unbearably slow.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev
ARG REVISION=dev

RUN apk add --no-cache upx

WORKDIR /workspace

# Cache module downloads before copying source.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

# Static, stripped, reproducible binary.
# The frontend (cmd/reaplet/static/) is embedded at compile time — no build step needed.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${REVISION}" \
    -o reaplet ./cmd/reaplet

RUN upx --best --lzma reaplet

# ---- Runtime --------------------------------------------------------------
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /workspace/reaplet /reaplet
EXPOSE 8080
ENTRYPOINT ["/reaplet"]
