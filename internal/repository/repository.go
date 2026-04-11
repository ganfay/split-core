package repository

import (
	"SplitCore/internal/domain"
	"context"
)

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) (*domain.User, error)
}

type FundRepository interface {
	Create(ctx context.Context, fund *domain.Fund) (*domain.Fund, error)
	GetByInviteCode(ctx context.Context, code string) (*domain.Fund, error)
	GetByUserID(ctx context.Context, userID int64, limit string, offset string) ([]domain.Fund, error)
}
