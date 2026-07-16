package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/rafaribe/reaplet/internal/infrastructure/cri"
	"github.com/rafaribe/reaplet/internal/infrastructure/k8s"
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

	criSocket := os.Getenv("CRI_SOCKET")

	// Infrastructure
	k8sClient, err := k8s.NewClient()
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		os.Exit(1)
	}

	// Repositories
	nodeRepo := k8s.NewNodeRepository(k8sClient)
	gcRepo := k8s.NewGCEventRepository(k8sClient)
	evictionRepo := k8s.NewEvictionRepository(k8sClient)
	imageRepo := cri.NewImageRepository(criSocket)

	// Use cases
	nodeUC := usecase.NewNodeUseCase(nodeRepo, gcRepo)
	actionUC := usecase.NewActionUseCase(evictionRepo, imageRepo)

	// HTTP layer
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	h := handler.NewHandler(nodeUC, actionUC)
	h.RegisterRoutes(r)

	// Serve embedded frontend
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		slog.Error("failed to create static filesystem", "error", err)
		os.Exit(1)
	}
	r.Handle("/*", http.FileServer(http.FS(staticFS)))

	slog.Info("starting reaplet", "port", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), r); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
