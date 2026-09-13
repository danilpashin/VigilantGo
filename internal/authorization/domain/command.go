package authorization

import "time"

type Transaction struct {
	AmountMinor     uint64
	Currency        string
	MerchantMCC     string
	MerchantCountry string
	Channel         string
	CardPresent     bool
	IPCountry       string
}

type RiskAssessment struct {
	Score       int
	ReasonCodes []string
}

type Verdict string

const (
	VerdictAllow  Verdict = "ALLOW"
	VerdictReview Verdict = "REVIEW"
	VerdictBlock  Verdict = "BLOCK"
)

type Decision struct {
	Verdict        Verdict
	Assessment     RiskAssessment
	RulesetVersion string
	Degraded       bool
	CreatedAt      time.Time
}
