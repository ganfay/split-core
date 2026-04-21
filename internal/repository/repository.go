package repository

import (
	"SplitCore/internal/domain"
	"context"
)

type UserRepository interface {
	CreateUser(ctx context.Context, u *domain.User) (*domain.User, error)
	GetUser(ctx context.Context, tgID int64) (*domain.User, error)
}

type FundRepository interface {
	CreateFund(ctx context.Context, fund *domain.Fund) (*domain.Fund, error)
	GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error)
	GetByUserID(ctx context.Context, userID int64, limit int, offset int) ([]domain.Fund, error)
	GetMembers(ctx context.Context, fundID int) ([]domain.User, error)

	AddMember(ctx context.Context, fund *domain.Fund, userID int64) error
	IsMember(ctx context.Context, fundID int, userID int64) (bool, error)
}

type PurchaseRepository interface {
	GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error)
	GetPurchasesByFundAll(ctx context.Context, fundID int) ([]domain.Purchase, error)
	CreatePurchase(ctx context.Context, purchase *domain.Purchase) error
}
