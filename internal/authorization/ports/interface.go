package ports

import (
	"context"
	"vigilantgo/internal/profile/domain"
)

type ProfileReader interface {
	GetProfile(ctx context.Context, cardToken, currency string) (*domain.Profile, error)
}
