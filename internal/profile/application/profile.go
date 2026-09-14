package application

import "vigilantgo/internal/profile/ports"

type Service struct {
	repo ports.ProfileRepository
}

func NewService(repo ports.ProfileRepository) *Service {
	return &Service{repo: repo}
}
