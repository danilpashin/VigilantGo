package application

import (
	"context"
	"time"
)

type AuthorizeCommand struct {
	SourceSystem    string
	AuthorizationID string
	OccurredAt      time.Time
	AmountMinor     uint64
	Currency        string
	CardToken       string
	Merchant        MerchantInput
	Channel         string
	POSEntryMode    string
	CardPresent     bool
	DeviceID        string
	IPCountry       string
	IdempotencyKey  string
	RequestID       string
}

type MerchantInput struct {
	MerchantID string
	Name       string
	MCC        string
	Country    string
	City       string
	TerminalID string
}

type AuthorizationResult struct {
	DecisionID      string
	AuthorizationID string
	Verdict         string
	RiskScore       int
	ReasonCodes     []string
	RulesetVersion  string
	Degraded        bool
	Replayed        bool
	ProcessingTime  time.Duration
}

type AuthorizationUseCase interface {
	Evaluate(ctx context.Context, cmd AuthorizeCommand) (AuthorizationResult, error)
}
