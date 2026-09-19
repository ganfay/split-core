package domain

import "time"

type Fund struct {
	ID         int       `json:"id" example:"1"`
	Name       string    `json:"name" example:"Trip to Paris"`
	AuthorID   int64     `json:"author_id" example:"1"`
	InviteCode string    `json:"invite_code" example:"aB12cD"`
	CreatedAt  time.Time `json:"created_at"`
}

type Pagination struct {
	Limit  int `json:"limit" example:"20"`
	Offset int `json:"offset" example:"0"`
}

type FundRequest struct {
	ID         int    `json:"id" example:"1"`
	InviteCode string `json:"invite_code,omitempty" example:"aB12cD"`
}

type JoinFundRequest struct {
	InviteCode string `json:"invite_code" example:"aB12cD"`
}

type FundPaginationRequest struct {
	FundID int `json:"fund_id" example:"1"`
	Limit  int `json:"limit" example:"20"`
	Offset int `json:"offset" example:"0"`
}

type VirtualUserRequest struct {
	FundID    int    `json:"fund_id" example:"1"`
	FirstName string `json:"first_name" example:"Alex"`
}

type RemoveUserRequest struct {
	FundID int   `json:"fund_id" example:"1"`
	UserID int64 `json:"user_id" example:"2"`
}

type ResponseError struct {
	Error string `json:"error"`
}
