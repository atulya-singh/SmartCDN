package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/at/smartcdn/internal/cache"
	"github.com/at/smartcdn/internal/config"
	"github.com/at/smartcdn/internal/handler"
	"github.com/at/smartcdn/internal/middleware"
	"github.com/at/smartcdn/internal/processor"
	"github.com/at/smartcdn/internal/storage"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()

	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	startTime := time.Now()

	store, err := storage.NewStorage(context.Background(), cfg)
	if err != nil {
		slog.Error("failed to initialize storage", "error", err)
		os.Exit(1)
	}

	imgCache := cache.NewCache(cfg)
	proc := processor.NewProcessor()

	mux := http.NewServeMux()
	mux.Handle("GET /health", handler.NewHealthHandler(startTime, map[string]handler.Pinger{
		"redis": imgCache,
		"minio": store,
	}))
	mux.Handle("POST /upload", handler.NewUploadHandler(store))
	mux.Handle("GET /img/{id}", handler.NewImageHandler(store, imgCache, proc))
	mux.Handle("GET /metrics", promhttp.Handler())

	wrapped := middleware.Logging(middleware.Metrics(mux))

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      wrapped,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("starting server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down server", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
