package main

import (
	"context"
	"encoding/json"
	"fmt"
	logger "go-antifraud/internal/logging"
	"go-antifraud/internal/platform/config"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	env := os.Getenv("APP_ENV")

	logger := logger.New(env)
	slog.SetDefault(logger)

	cfg := config.Load()

	runServer(cfg)
}

func runServer(cfg *config.Config) {
	r := newRouter()

	srv := http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler: r,
	}

	slog.Info("Server starting", "address", srv.Addr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	gracefulShutdown(&srv)
}

func newRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/health", healthHandler())

	return r
}

func gracefulShutdown(srv *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Forced shutdown", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Server exited gracefully")
}

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
