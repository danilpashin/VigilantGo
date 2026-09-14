package inhttp

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"

	"vigilantgo/internal/adapters/inbound/http/middleware"
	"vigilantgo/internal/authorization/application"

	"github.com/go-chi/chi"
	"github.com/go-playground/validator/v10"
)

type Handler struct {
	authorize application.AuthorizationUseCase
	validate  *validator.Validate
}

func NewHandler(authorize application.AuthorizationUseCase) *Handler {
	return &Handler{
		authorize: authorize,
		validate:  validator.New(),
	}
}

func (h *Handler) Evaluate(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if !isJSONContentType(r.Header.Get("Content-Type")) {
		writeError(w, http.StatusUnsupportedMediaType, ErrorResponse{
			Code:    "UNSUPPORTED_CONTENT_TYPE",
			Message: "Content-Type must be application/json",
		})
		return
	}

	var req AuthorizationRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrorResponse{
			Code:    "MALFORMED_JSON",
			Message: "request contains invalid JSON",
		})
		return
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, ErrorResponse{
			Code:    "MALFORMED_JSON",
			Message: "request must contain exactly one JSON object",
		})
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, ErrorResponse{
			Code:    "VALIDATION_FAILED",
			Message: "request contains invalid fields",
			Details: validationDetails(err),
		})
		return
	}

	if h.authorize == nil {
		writeError(w, http.StatusNotImplemented, ErrorResponse{
			Code:    "USE_CASE_NOT_CONFIGURED",
			Message: "authorization use case is not configured",
		})
		return
	}

	cmd := application.AuthorizeCommand{
		SourceSystem:    req.SourceSystem,
		AuthorizationID: req.AuthorizationID,
		OccurredAt:      req.OccurredAt,
		AmountMinor:     req.AmountMinor,
		Currency:        req.Currency,
		CardToken:       req.CardToken,
		Merchant: application.MerchantInput{
			MerchantID: req.Merchant.MerchantID,
			Name:       req.Merchant.Name,
			MCC:        req.Merchant.MCC,
			Country:    req.Merchant.Country,
			City:       req.Merchant.City,
			TerminalID: req.Merchant.TerminalID,
		},
		Channel:        req.Channel,
		POSEntryMode:   req.POSEntryMode,
		CardPresent:    *req.CardPresent,
		DeviceID:       req.DeviceID,
		IPCountry:      req.IPCountry,
		IdempotencyKey: middleware.IdempotencyKeyFromContext(r.Context()),
		RequestID:      middleware.RequestIDFromContext(r.Context()),
	}

	result, err := h.authorize.Evaluate(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, application.ErrInvalidCommand) {
			writeError(w, http.StatusBadRequest, ErrorResponse{
				Code:    "VALIDATION_FAILED",
				Message: "request contains invalid fields",
			})
			return
		}

		slog.ErrorContext(r.Context(), "authorization failed", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "authorization could not be processed",
		})
		return
	}

	writeAuthorizationResponse(w, http.StatusOK, AuthorizationResponse{
		DecisionID:      result.DecisionID,
		AuthorizationID: result.AuthorizationID,
		Verdict:         result.Verdict,
		RiskScore:       result.RiskScore,
		ReasonCodes:     result.ReasonCodes,
		RulesetVersion:  result.RulesetVersion,
		Degraded:        result.Degraded,
		Replayed:        result.Replayed,
		ProcessingTime:  result.ProcessingTime.Milliseconds(),
	})
}

func (h *Handler) SaveOutcome(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if !isJSONContentType(r.Header.Get("Content-Type")) {
		writeError(w, http.StatusUnsupportedMediaType, ErrorResponse{
			Code:    "UNSUPPORTED_CONTENT_TYPE",
			Message: "Content-Type must be application/json",
		})
		return
	}

	var req AuthorizationOutcomeRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrorResponse{
			Code:    "MALFORMED_JSON",
			Message: "request contains invalid JSON",
		})
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, ErrorResponse{
			Code:    "VALIDATION_FAILED",
			Message: "request contains invalid fields",
			Details: validationDetails(err),
		})
		return
	}

	if h.authorize == nil {
		writeError(w, http.StatusNotImplemented, ErrorResponse{
			Code:    "USE_CASE_NOT_CONFIGURED",
			Message: "authorization use case is not configured",
		})
		return
	}

	sourceSystem := chi.URLParam(r, "sourceSystem")
	authorizationID := chi.URLParam(r, "authorizationID")

	if sourceSystem == "" || authorizationID == "" {
		writeError(w, http.StatusBadRequest, ErrorResponse{
			Code:    "MISSING_VALUES",
			Message: "request must contain source system and authorization id",
		})
		return
	}

	cmd := application.OutcomeCommand{
		AuthorizationID:        authorizationID,
		SourceSystem:           sourceSystem,
		Status:                 req.Status,
		ProccessorResponseCode: req.ProcessorResponseCode,
		OccurredAt:             req.OccurredAt,
		IdempotencyKey:         middleware.IdempotencyKeyFromContext(r.Context()),
		RequestID:              middleware.RequestIDFromContext(r.Context()),
	}

	err := h.authorize.SaveOutcome(r.Context(), cmd)
	if err != nil {
		if errors.Is(err, application.ErrInvalidCommand) {
			writeError(w, http.StatusBadRequest, ErrorResponse{
				Code:    "VALIDATION_FAILED",
				Message: "request contains invalid fields",
			})
			return
		}

		slog.ErrorContext(r.Context(), "authorization outcome failed", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "authorization outcome could not be processed",
		})
		return
	}

	writeJSON(w, http.StatusOK, nil)
}

type ErrorResponse struct {
	Code      string             `json:"code"`
	Message   string             `json:"message"`
	RequestID string             `json:"request_id,omitempty"`
	Details   []ValidationDetail `json:"details,omitempty"`
}

type ValidationDetail struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

func writeAuthorizationResponse(w http.ResponseWriter, status int, response AuthorizationResponse) {
	writeJSON(w, status, response)
}

func writeError(w http.ResponseWriter, status int, response ErrorResponse) {
	writeJSON(w, status, response)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("encoding response", slog.Any("error", err))
	}
}

func isJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func validationDetails(err error) []ValidationDetail {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return []ValidationDetail{{Code: "INVALID_VALUE"}}
	}

	details := make([]ValidationDetail, 0, len(validationErrors))
	for _, fieldErr := range validationErrors {
		details = append(details, ValidationDetail{
			Field: fieldErr.Field(),
			Code:  fieldErr.Tag(),
		})
	}
	return details
}
