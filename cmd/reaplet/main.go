package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/rafaribe/reaplet/internal/infrastructure/k8s"
	"github.com/rafaribe/reaplet/internal/infrastructure/storage"
	"github.com/rafaribe/reaplet/internal/infrastructure/talos"
	handler "github.com/rafaribe/reaplet/internal/interfaces/http"
	"github.com/rafaribe/reaplet/internal/usecase"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	talosConfigPath := os.Getenv("TALOSCONFIG")
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/reaplet.db"
	}

	// Infrastructure — SQLite
	db, err := storage.New(dbPath)
	if err != nil {
		slog.Error("failed to open database", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Infrastructure — Kubernetes client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		os.Exit(1)
	}

	// Infrastructure — Talos API client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	talosClient, err := talos.NewClient(ctx, talosConfigPath)
	if err != nil {
		slog.Error("failed to create talos client", "error", err)
		os.Exit(1)
	}
	defer talosClient.Close() //nolint:errcheck

	// Repositories
	nodeRepo := k8s.NewNodeRepository(k8sClient)
	gcRepo := k8s.NewGCEventRepository(k8sClient)
	evictionRepo := k8s.NewEvictionRepository(k8sClient)
	imageRepo := talos.NewImageRepository(talosClient)

	// Use cases
	nodeUC := usecase.NewNodeUseCase(nodeRepo, gcRepo)
	nodeUC.SetImageAgeRepo(db)
	nodeUC.SetImageRepo(imageRepo)
	actionUC := usecase.NewActionUseCase(evictionRepo, imageRepo)

	// Feature use cases
	historyRec := usecase.NewHistoryRecorder(db, nodeUC)
	alertEngine := usecase.NewAlertEngine(db, nodeUC)
	cleanupEngine := usecase.NewCleanupEngine(db, nodeUC, actionUC)
	warmListUC := usecase.NewWarmListUseCase(db, nodeRepo)
	podStorageRepo := k8s.NewPodStorageRepository(k8sClient)

	// Background workers
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				historyRec.Record(ctx)
				alertEngine.Check(ctx)
			}
		}
	}()

	// Cleanup scheduler (check config interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Hour):
				cfg, err := db.GetCleanupConfig()
				if err != nil || !cfg.Enabled {
					continue
				}
				result, err := cleanupEngine.Run(ctx)
				if err != nil {
					slog.Error("scheduled cleanup failed", "error", err)
				} else if len(result.Removed) > 0 {
					slog.Info("scheduled cleanup completed", "removed", len(result.Removed), "dryRun", result.DryRun)
				}
			}
		}
	}()

	// HTTP layer
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(middleware.Timeout(30 * time.Second))

	if os.Getenv("DEV") == "true" {
		r.Use(handler.CORSMiddleware)
		slog.Info("CORS enabled for development")
	}

	// Core handlers
	h := handler.NewHandler(nodeUC, actionUC)
	h.RegisterRoutes(r)

	// Feature handlers
	fh := handler.NewFeaturesHandler(db, historyRec, alertEngine, cleanupEngine, nodeRepo, podStorageRepo, warmListUC)
	fh.RegisterRoutes(r)

	// Prometheus metrics
	r.Get("/metrics", handler.MetricsHandler(nodeUC))

	// Serve embedded frontend
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		slog.Error("failed to create static filesystem", "error", err)
		os.Exit(1)
	}
	r.Handle("/*", http.FileServer(http.FS(staticFS)))

	// Graceful shutdown
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("starting reaplet", "port", port, "db", dbPath)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	cancel() // stop background workers

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}

