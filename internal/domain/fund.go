package domain

import "time"

type Fund struct {
	ID         int64
	Name       string
	AuthorID   int64
	InviteCode string
	CreatedAt  time.Time
}
