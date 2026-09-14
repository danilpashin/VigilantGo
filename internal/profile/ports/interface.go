package ports

import (
	"context"
	"vigilantgo/internal/profile/domain"
)

type ProfileReader interface {
	GetProfile(ctx context.Context, cardToken, currency string) (*domain.Profile, error)
}

type ProfileWriter interface {
	SetProfile(ctx context.Context, profile domain.Profile) (bool, error)
}

type ProfileRepository interface {
	ProfileReader
	ProfileWriter
}
