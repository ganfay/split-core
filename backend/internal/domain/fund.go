package domain

import "time"

type Fund struct {
	ID         int
	Name       string
	AuthorID   int64
	InviteCode string
	CreatedAt  time.Time
}

type Pagination struct {
	Limit  int
	Offset int
}

type FundRequest struct {
	ID int
}
