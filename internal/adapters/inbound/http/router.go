package inhttp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
	inhttp "vigilantgo/internal/adapters/inbound/http/authorization"
	"vigilantgo/internal/adapters/inbound/http/middleware"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func NewRouter(authorizationHandler *inhttp.Handler, db *pgxpool.Pool, rdb *redis.Client) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestIDMiddleware)
	r.Use(middleware.PanicRecoveryMiddleware)
	r.Get("/health/live", livenessHandler())
	r.Get("/health/ready", readinessHandler(db, rdb))

	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.IdempotencyCheckMiddleware)
		r.Post("/authorizations/evaluate", authorizationHandler.Evaluate)
		r.Put("/authorizations/{sourceSystem}/{authorizationID}/outcome", authorizationHandler.SaveOutcome)
	})

	return r
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

func readinessHandler(db *pgxpool.Pool, cache *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]string{
			"db":    "UP",
			"cache": "UP",
		}
		isReady := true

		if err := db.Ping(ctx); err != nil {
			slog.ErrorContext(ctx, "Readiness check: DB ping failed", slog.Any("error", err))
			checks["db"] = "DOWN"
			isReady = false
		}

		if err := cache.Ping(ctx).Err(); err != nil {
			slog.ErrorContext(ctx, "Readiness check: Cache ping failed", slog.Any("error", err))
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
