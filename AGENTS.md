# AGENTS.md — Reaplet

## Project Overview

Reaplet is a lightweight storage monitor and garbage collector for Talos Kubernetes nodes. It provides a web UI showing per-node disk usage, container image inventories, kubelet GC events, storage forecasts, and allows direct image removal via the Talos API.

**Stack**: Go 1.26 backend (chi router) + vanilla HTML/JS/CSS frontend (no build step), compiled into a single `FROM scratch` container (~10MB).

## Architecture

Hexagonal (Ports & Adapters) with strict dependency inversion:

```
cmd/reaplet/main.go                → Composition root, wires everything
cmd/reaplet/static/                → Embedded frontend (index.html, app.js, style.css)
internal/domain/model/             → Business entities (no external deps)
internal/domain/repository/        → Port interfaces (outbound contracts)
internal/usecase/                  → Application logic (orchestrates ports)
internal/interfaces/http/          → Inbound adapter (chi HTTP handlers)
internal/infrastructure/k8s/       → Outbound adapter (Kubernetes API)
internal/infrastructure/talos/     → Outbound adapter (Talos gRPC API)
internal/infrastructure/storage/   → Outbound adapter (SQLite via modernc.org/sqlite)
internal/infrastructure/notify/    → Outbound adapter (Discord/Pushover webhooks)
deploy/                            → Kubernetes manifests (RBAC, Deployment, ServiceMonitor)
```

Dependency direction: `Adapters → UseCase → Domain`. Domain never imports infrastructure.

## Key Design Decisions

- **Single Deployment** (no DaemonSet): Talos API gives cluster-wide image management from one pod.
- **Embedded frontend**: Vanilla JS in `cmd/reaplet/static/`, embedded via Go's `//go:embed`. No build step.
- **Talos API for image removal**: Uses `MachineService.ImageClient.Remove` gRPC on target nodes via mTLS.
- **SQLite persistence**: Storage history, alert config, image age tracking, warm list. WAL mode.
- **No framework frontend**: Intentional — keeps the binary self-contained with zero npm dependencies. Revisit when component reuse pressure exceeds string-template ergonomics.

## Domain Model

Located in `internal/domain/model/model.go`:

| Entity | Purpose |
|--------|---------|
| `Node` | K8s node with storage info, image list |
| `StorageInfo` | Capacity/allocated/available bytes |
| `ContainerImage` | Image names, size, in-use flag |
| `GCEvent` | Kubelet garbage collection event metadata |
| `ImageRecommendation` | Unused image candidate with staleness (UnusedDays) |
| `PodStorageInfo` | Per-pod ephemeral storage usage |
| `ImageDuplicateGroup` | Duplicate images across nodes |
| `StorageForecast` | Linear regression projection to thresholds |
| `WarmListEntry/Status` | Pre-pull image management |
| `UpgradeCheckResult` | Talos upgrade disk budget per node |
| `EvictionRequest/Result` | Pod eviction request and outcome |
| `ImageRemovalRequest/Result` | Talos image removal request and outcome |
| `PreWarmCheckRequest/Result` | Image existence check on a node |

## Port Interfaces

Located in `internal/domain/repository/repository.go`:

| Port | Methods | Implemented By |
|------|---------|----------------|
| `NodeRepository` | `GetAll`, `GetByName` | `infrastructure/k8s.NodeRepository` |
| `GCEventRepository` | `GetRecentEvents`, `GetEventsForNode` | `infrastructure/k8s.GCEventRepository` |
| `PodEvictionRepository` | `Evict` | `infrastructure/k8s.EvictionRepository` |
| `ImageRepository` | `RemoveImage`, `ListImages` | `infrastructure/talos.ImageRepository` |
| `ImageAgeRepository` | `GetAllImageAges` | `infrastructure/storage.DB` |
| `PodStorageRepository` | `GetPodsOnNode` | `infrastructure/k8s.PodStorageRepository` |
| `WarmListRepository` | `GetWarmList`, `Add`, `Delete` | `infrastructure/storage.DB` |

## HTTP API

Core routes in `handler.go`, feature routes in `handler_features.go`:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Health check |
| GET | `/api/nodes` | All nodes with storage + images |
| GET | `/api/nodes/{name}` | Single node details |
| GET | `/api/nodes/{name}/history` | Storage history (range=1h\|24h\|7d) |
| GET | `/api/nodes/{name}/pods` | Per-pod ephemeral breakdown |
| GET | `/api/nodes/{name}/forecast` | Storage forecast (linear regression) |
| GET | `/api/gc-events` | Recent kubelet GC events |
| GET | `/api/recommendations` | Unused images sorted by staleness |
| POST | `/api/evict` | Evict a pod |
| POST | `/api/remove-image` | Remove image via Talos API |
| GET | `/api/alerts/config` | Alert configuration |
| PUT | `/api/alerts/config` | Update alert configuration |
| GET | `/api/alerts/history` | Past alert events |
| POST | `/api/alerts/test` | Send test notification |
| GET | `/api/cleanup/config` | Cleanup policy |
| PUT | `/api/cleanup/config` | Update cleanup policy |
| POST | `/api/cleanup/run` | Trigger manual cleanup |
| GET | `/api/cluster/summary` | Cluster-wide storage overview |
| GET | `/api/dedup` | Image deduplication report |
| GET | `/api/warm-list` | Warm list with missing status |
| POST | `/api/warm-list` | Add image to warm list |
| DELETE | `/api/warm-list/{id}` | Remove from warm list |
| POST | `/api/pre-warm-check` | Check image exists on node |
| GET | `/api/upgrade-check` | Talos upgrade disk budget |
| GET | `/metrics` | Prometheus metrics |

## Build & Run

```bash
# Build (no npm, no frontend step — it's just Go)
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
| `TALOSCONFIG` | `~/.talos/config` | Path to talosconfig for Talos API access |
| `DB_PATH` | `/data/reaplet.db` | SQLite database path |
| `DEV` | (unset) | Set to `true` to enable CORS for local development |

## Testing

- **Use case tests**: `internal/usecase/usecase_test.go` — Ginkgo/Gomega with fake repos
- **Handler tests**: `internal/interfaces/http/handler_test.go` — HTTP handler tests
- **Infrastructure tests**: `internal/infrastructure/k8s/*_test.go`, `storage/sqlite_test.go`

Run: `go test ./...`

## Deployment

Manifests in `deploy/`:

1. **`rbac.yaml`** — ServiceAccount + ClusterRole for node/pod/event access
2. **`server.yaml`** — Deployment (1 replica) + Service for the web UI/API
3. **`servicemonitor.yaml`** — Prometheus ServiceMonitor for /metrics

Namespace: `reaplet`. Talosconfig mounted as a Secret.

Deployed to home cluster via Flux HelmRelease (bjw-s app-template) in home-ops.

## Code Conventions

- Standard Go project layout (`cmd/`, `internal/`)
- Errors wrapped with `fmt.Errorf("context: %w", err)`
- Structured logging via `log/slog` (JSON handler)
- HTTP responses always JSON via `writeJSON`/`writeError` helpers
- No global state — all dependencies injected via constructors (except `GlobalCounters` for metrics)
- All new model types must have `json` struct tags
- Feature routes use direct path registration (`r.Get("/api/foo", ...)`) — NEVER nest inside `r.Route("/api", ...)` to avoid chi panics
- Frontend is vanilla HTML/JS/CSS — no build step, no npm, no framework

## CI/CD

- `.github/workflows/tests.yaml` — `go test ./...`
- `.github/workflows/lint.yaml` — golangci-lint
- `.github/workflows/release.yaml` — semantic version tag + GitHub Release + multi-arch container image

### Release Pipeline

Matrix build on native runners (no QEMU):
- `ubuntu-24.04` for amd64
- `ubuntu-24.04-arm` for arm64
- Manifest job merges per-arch images into single multi-arch tag
- Per-arch builds use `provenance: false` + `sbom: false` (required for manifest merge)

### CI Polling Guideline

When waiting for GitHub Actions pipelines (checks, release builds), **poll every 30 seconds** using `gh pr checks` or `gh run view`. Do not block indefinitely — use `sleep 30` between polls. Example pattern:

```bash
sleep 30 && gh pr checks <PR_NUMBER>
# or
sleep 30 && gh run view <RUN_ID> --json status,jobs --jq '{status, jobs: [.jobs[] | {name, status, conclusion}]}'
```

Continue polling until all jobs show `completed`. Only then proceed with merge or downstream actions.
