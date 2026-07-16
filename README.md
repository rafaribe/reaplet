# 🌾 Reaplet

A lightweight storage monitor for Talos Kubernetes nodes. Shows per-node disk usage, container image sizes, kubelet GC events, and lets you reclaim space by removing unused images directly via CRI.

## Architecture

- **Backend**: Go (chi router), talks to Kubernetes API + containerd CRI
- **Frontend**: Svelte 5, bundled as static assets embedded in the Go binary
- **Deployment**: Single `FROM scratch` image (~10-15MB) + privileged DaemonSet for CRI access

```
cmd/reaplet/          → Composition root (main.go)
internal/
  domain/model/       → Business entities (Node, Image, GCEvent)
  domain/repository/  → Port interfaces
  usecase/            → Application logic
  interfaces/http/    → HTTP handlers (chi)
  infrastructure/k8s/ → Kubernetes API adapter
  infrastructure/cri/ → Containerd CRI adapter
web/                  → Svelte frontend
deploy/               → Kubernetes manifests
```

## Features

- Per-node ephemeral storage usage with visual progress bars
- Full container image inventory with sizes and in-use status
- Kubelet GC event timeline
- Image removal recommendations (unused, oversized)
- Direct image removal via containerd CRI socket
- Pod eviction for storage pressure relief

## Build

```bash
# Frontend
cd web && npm ci && npm run build && cd ..

# Backend (builds with embedded frontend)
CGO_ENABLED=0 go build -ldflags='-s -w' -o reaplet ./cmd/reaplet

# Docker (produces ~10-15MB image)
docker build -t reaplet .
```

## Deploy

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/server.yaml
kubectl apply -f deploy/daemonset.yaml
```

## Development

```bash
# Run frontend dev server (hot reload)
cd web && npm run dev

# Run backend (needs kubeconfig)
go run ./cmd/reaplet
```
