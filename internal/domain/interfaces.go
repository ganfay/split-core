package domain

import (
	"context"

	tele "gopkg.in/telebot.v4"
)

type FundUsecase interface {
	GetBalance(ctx context.Context, fundID int) (*Settlement, error)
	AddExpense(ctx context.Context, c tele.Context, fundID int) (*Purchase, error)
}
