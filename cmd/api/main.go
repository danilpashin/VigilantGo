package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logger "go-antifraud/internal/logging"
	"go-antifraud/internal/platform/config"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	_ = godotenv.Load()

	env := os.Getenv("APP_ENV")

	logger := logger.New(env)
	slog.SetDefault(logger)

	cfg := config.Load()

	if err := runServer(cfg); err != nil {
		slog.Error("Application terminated with error", "error", err)
		os.Exit(1)
	}
}

func runServer(cfg *config.Config) error {
	db, err := initDB()
	if err != nil {
		return fmt.Errorf("init db: %w", err)
	}

	cache, err := initCache()
	if err != nil {
		_ = db.Close(context.Background())
		return fmt.Errorf("init cache: %w", err)
	}

	r := newRouter(db, cache)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("Server starting", "address", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	return gracefulShutdown(srv, db, cache, serverErr)
}

func newRouter(db *pgx.Conn, cache *redis.Client) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/health/live", livenessHandler())
	r.Get("/health/ready", readinessHandler(db, cache))

	return r
}

func initDB() (*pgx.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, os.Getenv("DB_URL"))
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	if err = conn.Ping(ctx); err != nil {
		_ = conn.Close(context.Background())
		return nil, fmt.Errorf("ping: %w", err)
	}

	return conn, nil
}

func initCache() (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_URL"),
		Password: "",
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return rdb, nil
}

func livenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "UP"})
	}
}

type HealthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func readinessHandler(db *pgx.Conn, cache *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{
			"db":    "UP",
			"cache": "UP",
		}
		isReady := true

		if err := db.Ping(ctx); err != nil {
			slog.ErrorContext(ctx, "Readiness check: DB ping failed", "error", err)
			checks["db"] = "DOWN"
			isReady = false
		}

		if err := cache.Ping(ctx).Err(); err != nil {
			slog.ErrorContext(ctx, "Readiness check: Cache ping failed", "error", err)
			checks["cache"] = "DOWN"
			isReady = false
		}

		status := http.StatusOK
		responseStatus := "UP"
		if !isReady {
			status = http.StatusServiceUnavailable
			responseStatus = "DOWN"
		}

		writeJSON(w, status, HealthResponse{
			Status: responseStatus,
			Checks: checks,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func gracefulShutdown(srv *http.Server, db *pgx.Conn, cache *redis.Client, serverErr <-chan error) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server error: %w", err)
	case sig := <-quit:
		slog.Info("Shutting down server...", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var shutdownErr error

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("HTTP shutdown error", "error", err)
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("srv shutdown: %w", err))
	}

	if err := cache.Close(); err != nil {
		slog.Error("Redis close error", "error", err)
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("redis close: %w", err))
	}

	if err := db.Close(ctx); err != nil {
		slog.Error("DB close error", "error", err)
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("db close: %w", err))
	}

	if shutdownErr == nil {
		slog.Info("Server stopped cleanly")
	}

	return shutdownErr
}
