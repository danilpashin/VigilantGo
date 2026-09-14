package domain

import "time"

const RulesetVersion = "2026-09-v1"

func EvaluateRisk(tx Transaction) RiskAssessment {
	assessment := RiskAssessment{}

	if tx.AmountMinor > 50_000_000 {
		assessment.Score += 500
		assessment.ReasonCodes = append(assessment.ReasonCodes, "AMOUNT_ABOVE_THRESHOLD")
	}

	if !tx.CardPresent && tx.AmountMinor > 10_000_000 {
		assessment.Score += 200
		assessment.ReasonCodes = append(assessment.ReasonCodes, "CARD_NOT_PRESENT_HIGH_AMOUNT")
	}

	if tx.IPCountry != "" && tx.MerchantCountry != "" && tx.IPCountry != tx.MerchantCountry {
		assessment.Score += 200
		assessment.ReasonCodes = append(assessment.ReasonCodes, "IP_COUNTRY_MISMATCH")
	}

	if assessment.Score > 1000 {
		assessment.Score = 1000
	}

	return assessment
}

func Decide(assessment RiskAssessment, now time.Time) Decision {
	verdict := VerdictAllow
	if assessment.Score >= 800 {
		verdict = VerdictBlock
	} else if assessment.Score >= 500 {
		verdict = VerdictReview
	}

	return Decision{
		Verdict:        verdict,
		Assessment:     assessment,
		RulesetVersion: RulesetVersion,
		CreatedAt:      now,
	}
}
