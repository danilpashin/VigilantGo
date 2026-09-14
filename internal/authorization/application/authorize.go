package application

import (
	"context"
	"errors"
	"time"
	"uuid"

	domain "vigilantgo/internal/authorization/domain"
	"vigilantgo/internal/authorization/ports"
)

var (
	ErrInvalidCommand       = errors.New("invalid authorization command")
	ErrProfileReaderMissing = errors.New("authorization profile reader is not configured")
	ErrOutcomeWriterMissing = errors.New("authorization outcome writer is not configured")
)

type Service struct {
	profileReader ports.ProfileReader
	outcomeWriter ports.OutcomeWriter
}

func NewService(profileReader ports.ProfileReader, outcomeWriter ports.OutcomeWriter) *Service {
	return &Service{profileReader: profileReader, outcomeWriter: outcomeWriter}
}

func (s *Service) Evaluate(ctx context.Context, cmd AuthorizeCommand) (AuthorizationResult, error) {
	startedAt := time.Now()

	if err := ctx.Err(); err != nil {
		return AuthorizationResult{}, err
	}
	if cmd.AuthorizationID == "" || cmd.SourceSystem == "" || cmd.AmountMinor == 0 {
		return AuthorizationResult{}, ErrInvalidCommand
	}
	if s.profileReader == nil {
		return AuthorizationResult{}, ErrProfileReaderMissing
	}

	transaction := domain.Transaction{
		AmountMinor:     cmd.AmountMinor,
		Currency:        cmd.Currency,
		MerchantMCC:     cmd.Merchant.MCC,
		MerchantCountry: cmd.Merchant.Country,
		Channel:         cmd.Channel,
		CardPresent:     cmd.CardPresent,
		IPCountry:       cmd.IPCountry,
	}

	assessment := domain.EvaluateRisk(transaction)
	decision := domain.Decide(assessment, time.Now().UTC())

	return AuthorizationResult{
		DecisionID:      uuid.New().String(),
		AuthorizationID: cmd.AuthorizationID,
		Verdict:         string(decision.Verdict),
		RiskScore:       decision.Assessment.Score,
		ReasonCodes:     decision.Assessment.ReasonCodes,
		RulesetVersion:  decision.RulesetVersion,
		Degraded:        decision.Degraded,
		Replayed:        false,
		ProcessingTime:  time.Since(startedAt),
	}, nil
}

func (s *Service) SaveOutcome(ctx context.Context, cmd OutcomeCommand) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if cmd.AuthorizationID == "" || cmd.SourceSystem == "" || cmd.Status == "" || cmd.RequestID == "" || cmd.IdempotencyKey == "" {
		return ErrInvalidCommand
	}

	authorizationID, err := uuid.Parse(cmd.AuthorizationID)
	if err != nil {
		return ErrInvalidCommand
	}
	if s.outcomeWriter == nil {
		return ErrOutcomeWriterMissing
	}

	var processorResponseCode *string
	if cmd.ProccessorResponseCode != "" {
		processorResponseCode = &cmd.ProccessorResponseCode
	}

	err = s.outcomeWriter.SaveOutcome(ctx, domain.AuthorizationOutcome{
		AuthorizationID:       authorizationID,
		Status:                domain.OutcomeStatus(cmd.Status),
		ProcessorResponseCode: processorResponseCode,
		Source:                cmd.SourceSystem,
		SourceEventID:         cmd.RequestID,
		IdempotencyKey:        cmd.IdempotencyKey,
		OccurredAt:            cmd.OccurredAt,
	})
	if err != nil {
		return err
	}

	return nil
}
