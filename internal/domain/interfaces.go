package domain

import (
	"context"
)

type FundUsecase interface {
	GetBalance(ctx context.Context, fundID int) (*Settlement, error)
	AddExpense(ctx context.Context, fundID int, id int64, desc string, cost float64) (*Purchase, error)

	CreateFund(ctx context.Context, fund *Fund) (*Fund, error)
	GetInfo(ctx context.Context, reqFund *Fund) (*Fund, error)
	GetByUserID(ctx context.Context, tgID int64, limit int, offset int) ([]Fund, error)
	AddMember(ctx context.Context, fund *Fund, tgID int64) error
	IsMember(ctx context.Context, fundID int, tgID int64) (bool, error)
	GetMembers(ctx context.Context, fundID int) ([]User, error)

	GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]Purchase, error)
	CreatePurchase(ctx context.Context, purchase *Purchase) error
}

type UserUsecase interface {
	GetOrCreateRealUser(ctx context.Context, tgID *int64, username string, firstName string) (int64, error)
	CreateVirtualUser(ctx context.Context, firstName string) (int64, error)
	GetUserByIID(ctx context.Context, iID int64) (*User, error)
}

type StatesUsecase interface {
	GetUserCtx(ctx context.Context, tgID *int64) (*UserContext, error)
	SaveUserCtx(ctx context.Context, tgID *int64, value *UserContext) error
}
