package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/rafaribe/reaplet/internal/domain/model"
	handler "github.com/rafaribe/reaplet/internal/interfaces/http"
	"github.com/rafaribe/reaplet/internal/usecase"
)

// --- Fakes ---

type fakeNodeRepo struct {
	nodes []model.Node
	err   error
}

func (f *fakeNodeRepo) GetAll(_ context.Context) ([]model.Node, error) {
	return f.nodes, f.err
}

func (f *fakeNodeRepo) GetByName(_ context.Context, name string) (*model.Node, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, n := range f.nodes {
		if n.Name == name {
			return &n, nil
		}
	}
	return nil, nil
}

type fakeGCRepo struct {
	events []model.GCEvent
	err    error
}

func (f *fakeGCRepo) GetRecentEvents(_ context.Context, _ int) ([]model.GCEvent, error) {
	return f.events, f.err
}

func (f *fakeGCRepo) GetEventsForNode(_ context.Context, _ string, _ int) ([]model.GCEvent, error) {
	return f.events, f.err
}

type fakeEvictionRepo struct {
	result *model.EvictionResult
	err    error
}

func (f *fakeEvictionRepo) Evict(_ context.Context, req model.EvictionRequest) (*model.EvictionResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &model.EvictionResult{PodName: req.PodName, Namespace: req.Namespace, Success: true}, nil
}

type fakeImageRepo struct {
	images []model.ContainerImage
	result *model.ImageRemovalResult
	err    error
}

func (f *fakeImageRepo) ListImages(_ context.Context, _ string) ([]model.ContainerImage, error) {
	return f.images, f.err
}

func (f *fakeImageRepo) RemoveImage(_ context.Context, req model.ImageRemovalRequest) (*model.ImageRemovalResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &model.ImageRemovalResult{ImageRef: req.ImageRef, NodeName: req.NodeName, Success: true, FreedBytes: 1024}, nil
}

// --- Helpers ---

func setupHandler(nodes []model.Node, gcEvents []model.GCEvent) *handler.Handler {
	nodeUC := usecase.NewNodeUseCase(
		&fakeNodeRepo{nodes: nodes},
		&fakeGCRepo{events: gcEvents},
	)
	actionUC := usecase.NewActionUseCase(
		&fakeEvictionRepo{},
		&fakeImageRepo{},
	)
	return handler.NewHandler(nodeUC, actionUC)
}

func setupRouter(h *handler.Handler) *chi.Mux {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

// --- Tests ---

func TestGetNodes_Success(t *testing.T) {
	nodes := []model.Node{
		{Name: "node-1", TotalImageSize: 1024},
		{Name: "node-2", TotalImageSize: 2048},
	}
	h := setupHandler(nodes, nil)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []model.Node
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result))
	}
}

func TestGetNodes_Error(t *testing.T) {
	nodeUC := usecase.NewNodeUseCase(
		&fakeNodeRepo{err: fmt.Errorf("k8s unreachable")},
		&fakeGCRepo{},
	)
	actionUC := usecase.NewActionUseCase(&fakeEvictionRepo{}, &fakeImageRepo{})
	h := handler.NewHandler(nodeUC, actionUC)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetNode_Success(t *testing.T) {
	nodes := []model.Node{{Name: "node-1", TotalImageSize: 5000}}
	h := setupHandler(nodes, nil)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/nodes/node-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result model.Node
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Name != "node-1" {
		t.Errorf("expected node-1, got %s", result.Name)
	}
}

func TestGetNode_NotFound(t *testing.T) {
	h := setupHandler([]model.Node{}, nil)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/nodes/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetGCEvents_Success(t *testing.T) {
	events := []model.GCEvent{
		{Reason: "ImageGCSucceeded", Message: "freed 200MB"},
	}
	h := setupHandler(nil, events)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/gc-events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetRecommendations_Success(t *testing.T) {
	nodes := []model.Node{
		{
			Name: "node-1",
			Images: []model.ContainerImage{
				{Names: []string{"unused:v1"}, SizeBytes: 100 * 1024 * 1024, InUse: false},
			},
		},
	}
	h := setupHandler(nodes, nil)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/recommendations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []model.ImageRecommendation
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(result))
	}
}

func TestEvictPod_Success(t *testing.T) {
	h := setupHandler(nil, nil)
	r := setupRouter(h)

	body := `{"PodName":"heavy-pod","Namespace":"default","NodeName":"node-1","Reason":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evict", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result model.EvictionResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestEvictPod_BadRequest(t *testing.T) {
	h := setupHandler(nil, nil)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/evict", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRemoveImage_Success(t *testing.T) {
	h := setupHandler(nil, nil)
	r := setupRouter(h)

	body := `{"NodeName":"node-1","ImageRef":"old:v1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/remove-image", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result model.ImageRemovalResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestRemoveImage_BadRequest(t *testing.T) {
	h := setupHandler(nil, nil)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/remove-image", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestContentTypeJSON(t *testing.T) {
	h := setupHandler([]model.Node{{Name: "n1"}}, nil)
	r := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}
