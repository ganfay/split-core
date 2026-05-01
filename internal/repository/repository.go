package repository

import (
	"context"

	"github.com/ganfay/split-core/internal/domain"
)

type UserRepository interface {
	GetOrCreateRealUser(ctx context.Context, tgID *int64, username, firstName string) (int64, error)
	CreateVirtualUser(ctx context.Context, firstName string) (int64, error)
	GetUserByIID(ctx context.Context, iID int64) (*domain.User, error)
}

type FundRepository interface {
	CreateFund(ctx context.Context, fund *domain.Fund) (*domain.Fund, error)
	GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error)
	GetByUserID(ctx context.Context, userID int64, limit int, offset int) ([]domain.Fund, error)
	GetMembers(ctx context.Context, fundID int) ([]domain.User, error)

	AddMember(ctx context.Context, fund *domain.Fund, userID int64) error
	IsMember(ctx context.Context, fundID int, IID int64) (bool, error)
}

type PurchaseRepository interface {
	GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error)
	GetPurchasesByFundAll(ctx context.Context, fundID int) ([]domain.Purchase, error)
	CreatePurchase(ctx context.Context, fundID int, amount float64, IID int64, desc string) error
}

type RedisRepository interface {
	GetUserCtx(ctx context.Context, userID *int64) (*domain.UserContext, error)
	SaveUserCtx(ctx context.Context, userID *int64, value *domain.UserContext) error
}
