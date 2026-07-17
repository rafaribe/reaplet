# 🌾 Reaplet

A lightweight storage monitor and garbage collector for Talos Kubernetes nodes. Shows per-node disk usage, container image sizes, kubelet GC events, and lets you reclaim space by removing unused images directly via the Talos API.

## Architecture

- **Backend**: Go (chi router), talks to Kubernetes API + Talos gRPC API
- **Frontend**: Vanilla HTML/JS/CSS (no build step, no framework, no dependencies)
- **Deployment**: Single `FROM scratch` image (~10MB), single Deployment (no DaemonSet)

```
cmd/reaplet/
  main.go              → Composition root
  static/              → Embedded frontend (index.html, app.js, style.css)
internal/
  domain/model/        → Business entities (Node, Image, GCEvent)
  domain/repository/   → Port interfaces
  usecase/             → Application logic
  interfaces/http/     → HTTP handlers (chi)
  infrastructure/k8s/  → Kubernetes API adapter
  infrastructure/talos/ → Talos API adapter (image list/remove)
deploy/                → Kubernetes manifests
```

## Features

- Per-node ephemeral storage usage with visual progress bars
- Full container image inventory with sizes and in-use status
- Kubelet GC event timeline
- Image removal recommendations (unused, oversized)
- Direct image removal via Talos API (no privileged DaemonSet needed)
- Pod eviction for storage pressure relief
- 5 color themes (dark, light, catppuccin, nord, dracula)
- Auto-refresh every 30s with live indicator

## Quick Start

```bash
# Create namespace and talosconfig secret
kubectl create namespace reaplet
kubectl -n reaplet create secret generic reaplet-talosconfig \
  --from-file=config=$HOME/.talos/config

# Deploy
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/server.yaml
```

## Build

```bash
# Build (no npm, no frontend step — it's just Go)
mise run build

# Docker
mise run docker-build

# Test
mise run test
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `TALOSCONFIG` | `~/.talos/config` | Path to talosconfig for Talos API access |
| `DEV` | (unset) | Set to `true` to enable CORS for local development |

## How It Works

1. **Node/image data**: Fetched from the Kubernetes API (`node.status.images`, ephemeral storage capacity)
2. **GC events**: Kubernetes events filtered for image GC reasons
3. **Image removal**: Calls the Talos `MachineService.ImageClient.Remove` gRPC RPC on the target node's apid
4. **Pod eviction**: Standard Kubernetes eviction API

The Talos API connection uses mTLS credentials from the mounted talosconfig secret. One Deployment can manage images on all nodes — no per-node DaemonSet required.

## Development

```bash
# Run backend with hot reload
mise run dev

# The frontend is plain HTML/JS/CSS in cmd/reaplet/static/
# Just edit and refresh — no build step
```

## Tech Stack

- Go 1.26 (chi router, slog, embed.FS)
- Talos machinery client v1.13.6
- Ginkgo v2 + Gomega (testing)
- Vanilla HTML/JS/CSS (5 themes, 546 lines total)
- Single `FROM scratch` Docker image
