package domain

import (
	"context"

	tele "gopkg.in/telebot.v4"
)

type FundUsecase interface {
	GetBalance(ctx context.Context, fundID int) (*Settlement, error)
	AddExpense(ctx context.Context, c tele.Context, fundID int) (*Purchase, error)

	CreateFund(ctx context.Context, fund *Fund) (*Fund, error)
	GetInfo(ctx context.Context, reqFund *Fund) (*Fund, error)
	GetByUserID(ctx context.Context, userID int64, limit int, offset int) ([]Fund, error)
	AddMember(ctx context.Context, fund *Fund, userID int64) error
	IsMember(ctx context.Context, fundID int, userID int64) (bool, error)
	GetMembers(ctx context.Context, fundID int) ([]User, error)

	GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]Purchase, error)
	CreatePurchase(ctx context.Context, purchase *Purchase) error
}

type UserUsecase interface {
	CreateUser(ctx context.Context, u *User) (*User, error)
	GetUser(ctx context.Context, tgID int64) (*User, error)
}
