# AGENTS.md — Reaplet

## Project Overview

Reaplet is a lightweight storage monitor and garbage collector for Talos Kubernetes nodes. It provides a web UI showing per-node disk usage, container image inventories, kubelet GC events, and allows direct image removal via the containerd CRI socket.

**Stack**: Go 1.26 backend (chi router) + Svelte 5 frontend, compiled into a single `FROM scratch` container (~10-15MB).

## Architecture

Hexagonal (Ports & Adapters) with strict dependency inversion:

```
cmd/reaplet/main.go            → Composition root, wires everything
internal/domain/model/          → Business entities (no external deps)
internal/domain/repository/     → Port interfaces (outbound contracts)
internal/usecase/               → Application logic (orchestrates ports)
internal/interfaces/http/       → Inbound adapter (chi HTTP handlers)
internal/infrastructure/k8s/    → Outbound adapter (Kubernetes API)
internal/infrastructure/cri/    → Outbound adapter (containerd CRI via gRPC)
web/                            → Svelte 5 SPA (built → embedded in Go binary)
deploy/                         → Kubernetes manifests (RBAC, Deployment, DaemonSet)
```

Dependency direction: `Adapters → UseCase → Domain`. Domain never imports infrastructure.

## Key Design Decisions

- **Two deployment components**: A central server Deployment (serves UI + cluster-wide K8s API calls) and a privileged DaemonSet agent per node (CRI socket access for image list/removal).
- **Embedded frontend**: Svelte builds to `cmd/reaplet/static/`, embedded via Go's `//go:embed`. Single binary serves everything.
- **CRI direct access**: Image removal bypasses kubelet — connects directly to containerd socket via CRI gRPC API (`k8s.io/cri-api`).
- **No database**: All data is live from Kubernetes API and CRI. No persistence layer.

## Domain Model

Located in `internal/domain/model/model.go`:

| Entity | Purpose |
|--------|---------|
| `Node` | K8s node with storage info, image list, last GC event |
| `StorageInfo` | Capacity/allocated/available bytes |
| `ContainerImage` | Image names, size, in-use flag |
| `GCEvent` | Kubelet garbage collection event metadata |
| `ImageRecommendation` | Unused image candidate for removal with reason |
| `EvictionRequest/Result` | Pod eviction request and outcome |
| `ImageRemovalRequest/Result` | CRI image removal request and outcome |

## Port Interfaces

Located in `internal/domain/repository/repository.go`:

| Port | Methods | Implemented By |
|------|---------|----------------|
| `NodeRepository` | `GetAll`, `GetByName` | `infrastructure/k8s.NodeRepository` |
| `GCEventRepository` | `GetRecentEvents`, `GetEventsForNode` | `infrastructure/k8s.GCEventRepository` |
| `PodEvictionRepository` | `Evict` | `infrastructure/k8s.EvictionRepository` |
| `ImageRepository` | `RemoveImage`, `ListImages` | `infrastructure/cri.ImageRepository` |

## Use Cases

Located in `internal/usecase/usecase.go`:

- **`NodeUseCase`** — monitoring: list nodes, get node details, recent GC events, image removal recommendations (sorted by size, unused images first).
- **`ActionUseCase`** — destructive ops: pod eviction, image removal via CRI.

## HTTP API

All routes under `/api`, registered in `internal/interfaces/http/handler.go`:

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/api/nodes` | `GetNodes` | All nodes with storage + images |
| GET | `/api/nodes/{name}` | `GetNode` | Single node details |
| GET | `/api/gc-events` | `GetGCEvents` | Recent kubelet GC events (limit 50) |
| GET | `/api/recommendations` | `GetRecommendations` | Unused image removal candidates |
| POST | `/api/evict` | `EvictPod` | Evict a pod (body: `EvictionRequest`) |
| POST | `/api/remove-image` | `RemoveImage` | Remove image via CRI (body: `ImageRemovalRequest`) |

## Build & Run

```bash
# Frontend
cd web && npm ci && npm run build && cd ..

# Backend (embeds built frontend)
CGO_ENABLED=0 go build -ldflags='-s -w' -o reaplet ./cmd/reaplet

# Docker (multi-stage, UPX compressed)
docker build -t reaplet .

# Tests
go test ./...
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `CRI_SOCKET` | `unix:///run/containerd/containerd.sock` | Containerd CRI socket path |
| `NODE_NAME` | (unset) | Set via downward API in DaemonSet |

## Testing

- **Use case tests**: `internal/usecase/usecase_test.go` — unit tests with mock repositories
- **Handler tests**: `internal/interfaces/http/handler_test.go` — HTTP handler tests
- **Infrastructure tests**: `internal/infrastructure/k8s/*_test.go` — adapter tests with fake K8s clients

Run: `go test ./...`

## Deployment Model

Three manifests in `deploy/`:

1. **`rbac.yaml`** — ServiceAccount + ClusterRole for node/pod/event access
2. **`server.yaml`** — Deployment (1 replica) + Service for the web UI/API
3. **`daemonset.yaml`** — Privileged DaemonSet on every node, mounts containerd socket

Namespace: `reaplet`

## Code Conventions

- Standard Go project layout (`cmd/`, `internal/`)
- Errors wrapped with `fmt.Errorf("context: %w", err)`
- Structured logging via `log/slog` (JSON handler)
- HTTP responses always JSON via `writeJSON`/`writeError` helpers
- No global state — all dependencies injected via constructors
- Frontend uses Svelte 5 with Vite bundler

## CI/CD

- `.github/workflows/tests.yaml` — runs `go test`
- `.github/workflows/lint.yaml` — linting
- `.github/workflows/release.yaml` — release pipeline
- `.renovaterc.json` — dependency update automation
- `release-please-config.json` — conventional commit releases
