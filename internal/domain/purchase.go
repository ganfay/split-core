package domain

import (
	"time"
)

type Purchase struct {
	ID          int
	FundID      int
	Payer       User
	Amount      float64
	Description string
	CreatedAt   time.Time
}

type Settlement struct {
	TotalAmount float64
	Average     float64
	Debts       []Debt
}

type Debt struct {
	FromID int64
	ToID   int64
	Amount float64
}

type Balance struct {
	UserID  int64
	Balance float64
}
