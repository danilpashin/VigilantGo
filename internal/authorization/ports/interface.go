package ports

import (
	"context"

	domain "vigilantgo/internal/authorization/domain"
	profiledomain "vigilantgo/internal/profile/domain"
)

type ProfileReader interface {
	GetProfile(ctx context.Context, cardToken, currency string) (*profiledomain.Profile, error)
}

type OutcomeReader interface{}

type OutcomeWriter interface {
	SaveOutcome(ctx context.Context, outcome domain.AuthorizationOutcome) error
}

type OutcomeRepository interface {
	OutcomeReader
	OutcomeWriter
}
