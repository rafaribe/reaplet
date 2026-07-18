package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rafaribe/reaplet/internal/domain/model"
	"github.com/rafaribe/reaplet/internal/usecase"
)

type Handler struct {
	nodeUC   *usecase.NodeUseCase
	actionUC *usecase.ActionUseCase
}

func NewHandler(nodeUC *usecase.NodeUseCase, actionUC *usecase.ActionUseCase) *Handler {
	return &Handler{
		nodeUC:   nodeUC,
		actionUC: actionUC,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", h.Health)
		r.Get("/nodes", h.GetNodes)
		r.Get("/nodes/{name}", h.GetNode)
		r.Get("/gc-events", h.GetGCEvents)
		r.Get("/recommendations", h.GetRecommendations)
		r.Post("/evict", h.EvictPod)
		r.Post("/remove-image", h.RemoveImage)
		r.Post("/remove-images-batch", h.RemoveImagesBatch)
	})
}

// Health returns a simple health check response.
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) GetNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.nodeUC.GetNodes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (h *Handler) GetNode(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	node, err := h.nodeUC.GetNode(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if node == nil {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (h *Handler) GetGCEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.nodeUC.GetRecentGCEvents(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *Handler) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	recs, err := h.nodeUC.RecommendImages(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *Handler) EvictPod(w http.ResponseWriter, r *http.Request) {
	var req model.EvictionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.actionUC.EvictPod(r.Context(), req)
	if err != nil {
		GlobalCounters.IncrPodEviction(req.NodeName, "failed")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result.Success {
		GlobalCounters.IncrPodEviction(req.NodeName, "success")
	} else {
		GlobalCounters.IncrPodEviction(req.NodeName, "failed")
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) RemoveImage(w http.ResponseWriter, r *http.Request) {
	var req model.ImageRemovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.actionUC.RemoveImage(r.Context(), req)
	if err != nil {
		GlobalCounters.IncrImageRemoval(req.NodeName, "failed")
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result.Success {
		GlobalCounters.IncrImageRemoval(req.NodeName, "success")
	} else {
		GlobalCounters.IncrImageRemoval(req.NodeName, "failed")
	}
	writeJSON(w, http.StatusOK, result)
}

// BatchRemovalRequest is the input for batch image removal.
type BatchRemovalRequest struct {
	Images []model.ImageRemovalRequest `json:"images"`
}

// BatchRemovalResponse is the output of batch image removal.
type BatchRemovalResponse struct {
	Succeeded int                        `json:"succeeded"`
	Failed    int                        `json:"failed"`
	FreedBytes int64                     `json:"freedBytes"`
	Results   []model.ImageRemovalResult `json:"results"`
}

// RemoveImagesBatch processes image removals sequentially to avoid memory spikes.
// Images are removed one at a time with the previous result fully released before
// starting the next, keeping memory stable even for large batches.
func (h *Handler) RemoveImagesBatch(w http.ResponseWriter, r *http.Request) {
	var req BatchRemovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Images) == 0 {
		writeError(w, http.StatusBadRequest, "no images specified")
		return
	}

	// Cap batch size to prevent abuse
	if len(req.Images) > 100 {
		req.Images = req.Images[:100]
	}

	resp := BatchRemovalResponse{
		Results: make([]model.ImageRemovalResult, 0, len(req.Images)),
	}

	// Process sequentially — one gRPC call at a time to keep memory stable
	for _, img := range req.Images {
		result, err := h.actionUC.RemoveImage(r.Context(), img)
		if err != nil {
			GlobalCounters.IncrImageRemoval(img.NodeName, "failed")
			resp.Failed++
			resp.Results = append(resp.Results, model.ImageRemovalResult{
				ImageRef: img.ImageRef,
				NodeName: img.NodeName,
				Success:  false,
				Error:    err.Error(),
			})
			continue
		}

		if result.Success {
			GlobalCounters.IncrImageRemoval(img.NodeName, "success")
			resp.Succeeded++
			resp.FreedBytes += result.FreedBytes
		} else {
			GlobalCounters.IncrImageRemoval(img.NodeName, "failed")
			resp.Failed++
		}
		resp.Results = append(resp.Results, *result)
	}

	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// CORSMiddleware allows cross-origin requests in development.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
