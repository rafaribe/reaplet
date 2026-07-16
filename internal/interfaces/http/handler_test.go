package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-chi/chi/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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

// --- Specs ---

var _ = Describe("HTTP Handler", func() {
	Describe("GET /api/nodes", func() {
		It("returns all nodes with 200", func() {
			nodes := []model.Node{
				{Name: "node-1", TotalImageSize: 1024},
				{Name: "node-2", TotalImageSize: 2048},
			}
			h := setupHandler(nodes, nil)
			r := setupRouter(h)

			req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var result []model.Node
			Expect(json.NewDecoder(w.Body).Decode(&result)).To(Succeed())
			Expect(result).To(HaveLen(2))
		})

		It("returns 500 when repository fails", func() {
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

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("returns Content-Type application/json", func() {
			h := setupHandler([]model.Node{{Name: "n1"}}, nil)
			r := setupRouter(h)

			req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))
		})
	})

	Describe("GET /api/nodes/{name}", func() {
		It("returns the node with 200", func() {
			nodes := []model.Node{{Name: "node-1", TotalImageSize: 5000}}
			h := setupHandler(nodes, nil)
			r := setupRouter(h)

			req := httptest.NewRequest(http.MethodGet, "/api/nodes/node-1", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var result model.Node
			Expect(json.NewDecoder(w.Body).Decode(&result)).To(Succeed())
			Expect(result.Name).To(Equal("node-1"))
		})

		It("returns 404 when node not found", func() {
			h := setupHandler([]model.Node{}, nil)
			r := setupRouter(h)

			req := httptest.NewRequest(http.MethodGet, "/api/nodes/nonexistent", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("GET /api/gc-events", func() {
		It("returns events with 200", func() {
			events := []model.GCEvent{
				{Reason: "ImageGCSucceeded", Message: "freed 200MB"},
			}
			h := setupHandler(nil, events)
			r := setupRouter(h)

			req := httptest.NewRequest(http.MethodGet, "/api/gc-events", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GET /api/recommendations", func() {
		It("returns recommendations with 200", func() {
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

			Expect(w.Code).To(Equal(http.StatusOK))

			var result []model.ImageRecommendation
			Expect(json.NewDecoder(w.Body).Decode(&result)).To(Succeed())
			Expect(result).To(HaveLen(1))
		})
	})

	Describe("POST /api/evict", func() {
		It("evicts a pod successfully", func() {
			h := setupHandler(nil, nil)
			r := setupRouter(h)

			body := `{"PodName":"heavy-pod","Namespace":"default","NodeName":"node-1","Reason":"test"}`
			req := httptest.NewRequest(http.MethodPost, "/api/evict", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var result model.EvictionResult
			Expect(json.NewDecoder(w.Body).Decode(&result)).To(Succeed())
			Expect(result.Success).To(BeTrue())
		})

		It("returns 400 for invalid JSON body", func() {
			h := setupHandler(nil, nil)
			r := setupRouter(h)

			req := httptest.NewRequest(http.MethodPost, "/api/evict", strings.NewReader("invalid json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("POST /api/remove-image", func() {
		It("removes an image successfully", func() {
			h := setupHandler(nil, nil)
			r := setupRouter(h)

			body := `{"NodeName":"node-1","ImageRef":"old:v1"}`
			req := httptest.NewRequest(http.MethodPost, "/api/remove-image", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var result model.ImageRemovalResult
			Expect(json.NewDecoder(w.Body).Decode(&result)).To(Succeed())
			Expect(result.Success).To(BeTrue())
		})

		It("returns 400 for invalid JSON body", func() {
			h := setupHandler(nil, nil)
			r := setupRouter(h)

			req := httptest.NewRequest(http.MethodPost, "/api/remove-image", strings.NewReader("{invalid"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})
})
