package inhttp

import "time"

type AuthorizationRequest struct {
	SourceSystem    string    `json:"source_system" validate:"required"`
	AuthorizationID string    `json:"authorization_id" validate:"required"`
	OccurredAt      time.Time `json:"occurred_at" validate:"required"`
	AmountMinor     uint64    `json:"amount_minor" validate:"required,gt=0"`
	Currency        string    `json:"currency" validate:"required,len=3,uppercase"`
	CardToken       string    `json:"card_token" validate:"required"`
	Merchant        Merchant  `json:"merchant" validate:"required"`
	Channel         string    `json:"channel" validate:"required,oneof=ECOM POS ATM"`
	POSEntryMode    string    `json:"pos_entry_mode" validate:"required"`
	CardPresent     *bool     `json:"card_present" validate:"required"`
	DeviceID        string    `json:"device_id" validate:"required"`
	IPCountry       string    `json:"ip_country" validate:"omitempty,len=2,uppercase"`
}

type AuthorizationResponse struct {
	DecisionID      string   `json:"decision_id"`
	AuthorizationID string   `json:"authorization_id"`
	Verdict         string   `json:"verdict"`
	RiskScore       int      `json:"risk_score"`
	ReasonCodes     []string `json:"reason_codes"`
	RulesetVersion  string   `json:"ruleset_version"`
	Degraded        bool     `json:"degraded"`
	Replayed        bool     `json:"replayed"`
	ProcessingTime  int64    `json:"processing_time_ms"`
}

type Merchant struct {
	MerchantID string `json:"merchant_id" validate:"required"`
	Name       string `json:"name" validate:"required"`
	MCC        string `json:"mcc" validate:"required,len=4,numeric"`
	Country    string `json:"country" validate:"required,len=2,uppercase"`
	City       string `json:"city" validate:"required"`
	TerminalID string `json:"terminal_id" validate:"required"`
}

type AuthorizationOutcomeRequest struct {
	Status                string    `json:"status" validate:"required,uppercase"`
	ProcessorResponseCode string    `json:"processor_response_code" validate:"required,len=2"`
	OccurredAt            time.Time `json:"occurred_at" validate:"required"`
}
