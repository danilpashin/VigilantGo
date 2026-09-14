package application

import (
	"context"
	"errors"
	"time"
	"uuid"

	domain "vigilantgo/internal/authorization/domain"
	"vigilantgo/internal/profile/ports"
)

var ErrInvalidCommand = errors.New("invalid authorization command")

type Service struct {
	profileReader ports.ProfileReader
}

func NewService(profileReader ports.ProfileReader) *Service {
	return &Service{profileReader: profileReader}
}

func (s *Service) Evaluate(ctx context.Context, cmd AuthorizeCommand) (AuthorizationResult, error) {
	startedAt := time.Now()

	if err := ctx.Err(); err != nil {
		return AuthorizationResult{}, err
	}
	if cmd.AuthorizationID == "" || cmd.SourceSystem == "" || cmd.AmountMinor == 0 {
		return AuthorizationResult{}, ErrInvalidCommand
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
