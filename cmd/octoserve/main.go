package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jbpratt/octoserve/internal/api"
	"github.com/jbpratt/octoserve/internal/api/handlers"
	"github.com/jbpratt/octoserve/internal/api/middleware"
	"github.com/jbpratt/octoserve/internal/config"
	"github.com/jbpratt/octoserve/internal/storage/filesystem"
)

func main() {
	// Parse command line flags
	var (
		configFile = flag.String("config", "config.json", "Path to configuration file")
		showHelp   = flag.Bool("help", false, "Show help message")
		version    = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}

	if *version {
		fmt.Println("octoserve v0.1.0 - Minimal OCI Registry")
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Override with environment variables
	cfg.LoadFromEnv()

	// Setup logger
	logger := setupLogger(cfg.Logging)
	logger.Info("Starting octoserve", "version", "0.1.0", "config_file", *configFile)

	// Initialize storage backend
	store, err := filesystem.New(cfg.Storage.Path)
	if err != nil {
		logger.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	logger.Info("Storage initialized", "type", cfg.Storage.Type, "path", cfg.Storage.Path)

	// Create handlers
	registryHandler := handlers.NewRegistryHandler(store)
	blobHandler := handlers.NewBlobHandler(store)
	manifestHandler := handlers.NewManifestHandler(store)
	uploadHandler := handlers.NewUploadHandler(store)

	// Setup router
	router := api.NewRouter()

	// Registry API endpoints
	router.GET("/v2/", registryHandler.CheckAPI)

	// Blob endpoints
	router.GET("/v2/{name}/blobs/{digest}", blobHandler.GetBlob)
	router.HEAD("/v2/{name}/blobs/{digest}", blobHandler.HeadBlob)
	router.DELETE("/v2/{name}/blobs/{digest}", blobHandler.DeleteBlob)

	// Upload endpoints
	router.POST("/v2/{name}/blobs/uploads/", uploadHandler.StartUpload)
	router.PATCH("/v2/{name}/blobs/uploads/{uuid}", uploadHandler.PatchUpload)
	router.PUT("/v2/{name}/blobs/uploads/{uuid}", uploadHandler.PutUpload)
	router.GET("/v2/{name}/blobs/uploads/{uuid}", uploadHandler.GetUpload)
	router.DELETE("/v2/{name}/blobs/uploads/{uuid}", uploadHandler.DeleteUpload)

	// Manifest endpoints
	router.GET("/v2/{name}/manifests/{reference}", manifestHandler.GetManifest)
	router.HEAD("/v2/{name}/manifests/{reference}", manifestHandler.HeadManifest)
	router.PUT("/v2/{name}/manifests/{reference}", manifestHandler.PutManifest)
	router.DELETE("/v2/{name}/manifests/{reference}", manifestHandler.DeleteManifest)

	// Tag listing
	router.GET("/v2/{name}/tags/list", manifestHandler.ListTags)

	// Apply middleware
	handler := middleware.Chain(
		middleware.SetCORSHeaders,
		middleware.SetDockerHeaders,
		middleware.RequestLogger(logger),
		middleware.ValidateRepository,
		middleware.ValidateReference,
		middleware.ValidateDigest,
	)(router)

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Starting HTTP server", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server exited")
}

// setupLogger creates a structured logger based on configuration
func setupLogger(cfg config.LoggingConfig) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}