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

	// Infrastructure — Kubernetes client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		os.Exit(1)
	}

	// Infrastructure — Talos API client
	ctx := context.Background()
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
	actionUC := usecase.NewActionUseCase(evictionRepo, imageRepo)

	// HTTP layer
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(middleware.Timeout(30 * time.Second))

	// Enable CORS in dev mode
	if os.Getenv("DEV") == "true" {
		r.Use(handler.CORSMiddleware)
		slog.Info("CORS enabled for development")
	}

	h := handler.NewHandler(nodeUC, actionUC)
	h.RegisterRoutes(r)

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
		slog.Info("starting reaplet", "port", port, "talosconfig", talosConfigPath)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}
