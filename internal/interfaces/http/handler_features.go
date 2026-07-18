package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
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
}

func NewFeaturesHandler(
	db *storage.DB,
	historyRec *usecase.HistoryRecorder,
	alertEngine *usecase.AlertEngine,
	cleanupEngine *usecase.CleanupEngine,
	nodeRepo repository.NodeRepository,
) *FeaturesHandler {
	return &FeaturesHandler{
		db:            db,
		historyRec:    historyRec,
		alertEngine:   alertEngine,
		cleanupEngine: cleanupEngine,
		nodeRepo:      nodeRepo,
	}
}

func (fh *FeaturesHandler) RegisterRoutes(r chi.Router) {
	// Storage history
	r.Get("/api/nodes/{name}/history", fh.GetNodeHistory)

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
