package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rafaribe/reaplet/internal/domain/model"
	"github.com/rafaribe/reaplet/internal/domain/repository"
	"github.com/rafaribe/reaplet/internal/infrastructure/storage"
	"github.com/rafaribe/reaplet/internal/usecase"
)

// FeaturesHandler handles all new feature endpoints.
type FeaturesHandler struct {
	db            *storage.DB
	historyRec    *usecase.HistoryRecorder
	alertEngine   *usecase.AlertEngine
	cleanupEngine *usecase.CleanupEngine
	nodeRepo      repository.NodeRepository
	podStorageRepo repository.PodStorageRepository
	warmListUC    *usecase.WarmListUseCase
}

func NewFeaturesHandler(
	db *storage.DB,
	historyRec *usecase.HistoryRecorder,
	alertEngine *usecase.AlertEngine,
	cleanupEngine *usecase.CleanupEngine,
	nodeRepo repository.NodeRepository,
	podStorageRepo repository.PodStorageRepository,
	warmListUC *usecase.WarmListUseCase,
) *FeaturesHandler {
	return &FeaturesHandler{
		db:             db,
		historyRec:     historyRec,
		alertEngine:    alertEngine,
		cleanupEngine:  cleanupEngine,
		nodeRepo:       nodeRepo,
		podStorageRepo: podStorageRepo,
		warmListUC:     warmListUC,
	}
}

func (fh *FeaturesHandler) RegisterRoutes(r chi.Router) {
	// Storage history
	r.Get("/api/nodes/{name}/history", fh.GetNodeHistory)

	// Pod storage breakdown
	r.Get("/api/nodes/{name}/pods", fh.GetNodePods)

	// Storage forecast
	r.Get("/api/nodes/{name}/forecast", fh.GetNodeForecast)

	// Alert config
	r.Get("/api/alerts/config", fh.GetAlertConfig)
	r.Put("/api/alerts/config", fh.UpdateAlertConfig)
	r.Get("/api/alerts/history", fh.GetAlertHistory)
	r.Post("/api/alerts/test", fh.TestAlert)

	// Cleanup
	r.Get("/api/cleanup/config", fh.GetCleanupConfig)
	r.Put("/api/cleanup/config", fh.UpdateCleanupConfig)
	r.Post("/api/cleanup/run", fh.RunCleanup)

	// Cluster summary
	r.Get("/api/cluster/summary", fh.GetClusterSummary)

	// Image deduplication
	r.Get("/api/dedup", fh.GetDedup)

	// Warm list
	r.Get("/api/warm-list", fh.GetWarmList)
	r.Post("/api/warm-list", fh.AddWarmList)
	r.Delete("/api/warm-list/{id}", fh.DeleteWarmList)

	// Pre-warm check
	r.Post("/api/pre-warm-check", fh.PreWarmCheck)

	// Upgrade check
	r.Get("/api/upgrade-check", fh.GetUpgradeCheck)
}

// --- History ---

func (fh *FeaturesHandler) GetNodeHistory(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "24h"
	}

	points, err := fh.historyRec.GetHistory(name, rangeStr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, points)
}

// --- Alerts ---

func (fh *FeaturesHandler) GetAlertConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := fh.db.GetAlertConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (fh *FeaturesHandler) UpdateAlertConfig(w http.ResponseWriter, r *http.Request) {
	var cfg storage.AlertConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := fh.db.SaveAlertConfig(&cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (fh *FeaturesHandler) GetAlertHistory(w http.ResponseWriter, r *http.Request) {
	events, err := fh.db.GetAlertEvents(50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (fh *FeaturesHandler) TestAlert(w http.ResponseWriter, r *http.Request) {
	if err := fh.alertEngine.SendTest(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// --- Cleanup ---

func (fh *FeaturesHandler) GetCleanupConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := fh.db.GetCleanupConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (fh *FeaturesHandler) UpdateCleanupConfig(w http.ResponseWriter, r *http.Request) {
	var cfg storage.CleanupConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := fh.db.SaveCleanupConfig(&cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (fh *FeaturesHandler) RunCleanup(w http.ResponseWriter, r *http.Request) {
	result, err := fh.cleanupEngine.Run(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Cluster ---

func (fh *FeaturesHandler) GetClusterSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := usecase.GetClusterSummary(r.Context(), fh.nodeRepo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// --- Pod Storage Breakdown ---

func (fh *FeaturesHandler) GetNodePods(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	pods, err := fh.podStorageRepo.GetPodsOnNode(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pods)
}

// --- Storage Forecast ---

func (fh *FeaturesHandler) GetNodeForecast(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	forecast, err := usecase.GetStorageForecast(fh.db, fh.nodeRepo, r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, forecast)
}

// --- Image Deduplication ---

func (fh *FeaturesHandler) GetDedup(w http.ResponseWriter, r *http.Request) {
	groups, err := usecase.GetImageDeduplication(r.Context(), fh.nodeRepo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

// --- Warm List ---

func (fh *FeaturesHandler) GetWarmList(w http.ResponseWriter, r *http.Request) {
	status, err := fh.warmListUC.GetStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (fh *FeaturesHandler) AddWarmList(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ImageRef string `json:"imageRef"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ImageRef == "" {
		writeError(w, http.StatusBadRequest, "invalid request body: imageRef required")
		return
	}
	entry, err := fh.warmListUC.Add(req.ImageRef)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (fh *FeaturesHandler) DeleteWarmList(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := fh.warmListUC.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Pre-Warm Check ---

func (fh *FeaturesHandler) PreWarmCheck(w http.ResponseWriter, r *http.Request) {
	var req model.PreWarmCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ImageRef == "" || req.NodeName == "" {
		writeError(w, http.StatusBadRequest, "imageRef and nodeName are required")
		return
	}
	result, err := usecase.PreWarmCheck(r.Context(), fh.nodeRepo, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Upgrade Check ---

func (fh *FeaturesHandler) GetUpgradeCheck(w http.ResponseWriter, r *http.Request) {
	results, err := usecase.GetUpgradeCheck(r.Context(), fh.nodeRepo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}
