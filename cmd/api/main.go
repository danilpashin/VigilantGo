package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	router "vigilantgo/internal/adapters/inbound/http"
	inhttp "vigilantgo/internal/adapters/inbound/http/authorization"
	authorizationapp "vigilantgo/internal/authorization/application"
	logger "vigilantgo/internal/logging"
	"vigilantgo/internal/platform/cache"
	"vigilantgo/internal/platform/config"
	"vigilantgo/internal/platform/database"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	env := os.Getenv("APP_ENV")

	logger := logger.New(env)
	slog.SetDefault(logger)

	if migrateCLI() {
		return
	}

	if err := runServer(cfg); err != nil {
		slog.Error("Application terminated with error", "error", err)
		os.Exit(1)
	}
}

func runServer(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.NewPool(ctx, os.Getenv("APP_DB_URL"))
	if err != nil {
		return fmt.Errorf("init db: %w", err)
	}

	rdb, err := cache.NewClient(ctx, os.Getenv("REDIS_URL"))
	if err != nil {
		db.Close()
		return fmt.Errorf("init cache: %w", err)
	}

	useCase := authorizationapp.NewService()
	handler := inhttp.NewHandler(useCase)

	r := router.NewRouter(handler, db, rdb)

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

	return gracefulShutdown(srv, db, rdb, serverErr)
}

func migrateCLI() bool {
	var cmd string
	var version int

	flag.StringVar(&cmd, "cmd", "", "migration command: up, down, force, version")
	flag.IntVar(&version, "version", version, "version for force or migrate command")
	flag.Parse()

	if cmd == "" {
		return false
	}

	if err := database.RunMigrations(cmd, version); err != nil {
		slog.Error("Failed to migrate", "error", err)
	}
	slog.Info("Migration completed")

	return true
}

func gracefulShutdown(srv *http.Server, db *pgxpool.Pool, cache *redis.Client, serverErr <-chan error) error {
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
		slog.Error("HTTP shutdown error", slog.Any("error", err))
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("srv shutdown: %w", err))
	}

	if err := cache.Close(); err != nil {
		slog.Error("Redis close error", slog.Any("error", err))
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("redis close: %w", err))
	}

	db.Close()

	if shutdownErr == nil {
		slog.Info("Server stopped cleanly")
	}

	return shutdownErr
}
