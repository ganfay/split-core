package domain

import (
	"time"
)

type Purchase struct {
	ID          int       `json:"id" example:"1"`
	FundID      int       `json:"fund_id" example:"1"`
	Payer       User      `json:"payer"`
	Amount      float64   `json:"amount" example:"150.5"`
	Description string    `json:"description" example:"Taxi to hotel"`
	CreatedAt   time.Time `json:"created_at"`
}

type Settlement struct {
	TotalAmount float64 `json:"total_amount" example:"451.5"`
	Average     float64 `json:"average" example:"150.5"`
	Debts       []Debt  `json:"debts"`
}

type Debt struct {
	FromID int64   `json:"from_id" example:"2"`
	ToID   int64   `json:"to_id" example:"1"`
	Amount float64 `json:"amount" example:"50.25"`
}

type Balance struct {
	UserID  int64   `json:"user_id" example:"1"`
	Balance float64 `json:"balance" example:"50.25"`
}

type SettleFund struct {
	FundID      int     `json:"fund_id" example:"1"`
	Description string  `json:"description" example:"Taxi to hotel"`
	Cost        float64 `json:"cost" example:"150.5"`
}
