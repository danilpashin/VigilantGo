package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"uuid"
)

func PanicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("Panic recovered", slog.Any("error", err))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func IdempotencyCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" || len(key) > 255 {
			http.Error(w, "invalid Idempotency-Key", http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), idempotencyKey, key)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type contextKey string

const (
	requestIDKey   contextKey = "request_id"
	idempotencyKey contextKey = "idempotency_key"
)

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if reqID == "" {
			reqID = uuid.New().String()
		}

		w.Header().Set("X-Request-ID", reqID)

		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func IdempotencyKeyFromContext(ctx context.Context) string {
	value, _ := ctx.Value(idempotencyKey).(string)
	return value
}
