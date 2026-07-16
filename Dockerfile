# syntax=docker/dockerfile:1

ARG GO_VERSION=1.23
ARG NODE_VERSION=22

# ---- UI build -------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS ui
WORKDIR /ui
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- Go build -------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS builder

ARG TARGETOS
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

# Overlay the freshly built UI into the embedded static dir.
COPY --from=ui /ui/../cmd/reaplet/static/ cmd/reaplet/static/

# Static, stripped, reproducible binary.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
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
