package repository

import (
	"SplitCore/internal/domain"
	"context"
)

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) (*domain.User, error)
	Get(ctx context.Context, tgID int64) (*domain.User, error)
}

type FundRepository interface {
	Create(ctx context.Context, fund *domain.Fund) (*domain.Fund, error)
	GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error)
	GetByUserID(ctx context.Context, userID int64, limit int, offset int) ([]domain.Fund, error)

	AddMember(ctx context.Context, fund *domain.Fund, userID int64) error

	GetPurchasesByFund(ctx context.Context, fund *domain.Fund) ([]domain.Purchase, error)
	CreatePurchase(ctx context.Context, purchase *domain.Purchase) error
}
