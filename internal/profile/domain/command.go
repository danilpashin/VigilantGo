package domain

import "time"

type Profile struct {
	CustomerID                 string
	Currency                   string
	ProfileVersion             int64
	SuccessfulTransactionCount int64
	TotalAmountMinor           int64
	MeanAmountMinor            string
	AmountM2                   string
	FirstSuccessfulAt          *time.Time
	LastSuccessfulAt           *time.Time
	UpdatedAt                  time.Time
	MCCStats                   []ProfileMCC
	CountryStats               []ProfileCountry
}

type ProfileMCC struct {
	CustomerID       string
	Currency         string
	MCC              string
	TransactionCount int64
	TotalAmountMinor int64
	LastSeenAt       *time.Time
	UpdatedAt        time.Time
}

type ProfileCountry struct {
	CustomerID       string
	Currency         string
	Country          string
	TransactionCount int64
	TotalAmountMinor int64
	LastSeenAt       *time.Time
	UpdatedAt        time.Time
}
