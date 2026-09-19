package domain

import (
	"fmt"
	"time"
)

type User struct {
	ID        int64     `json:"id" example:"1"`
	TgID      *int64    `json:"tg_id,omitempty" example:"123456789"`
	Username  string    `json:"username" example:"ganfay"`
	FirstName string    `json:"first_name" example:"Alex"`
	IsVirtual bool      `json:"is_virtual" example:"false"`
	CreatedAt time.Time `json:"created_at"`
}

func (u User) GetDisplayName() string {
	if u.Username != "" {
		return "@" + u.Username
	}
	if u.FirstName != "" && u.FirstName != "." {
		return u.FirstName
	}
	if u.TgID != nil {
		return fmt.Sprintf("User_%d", *u.TgID%10000)
	}
	return fmt.Sprintf("User_%d", u.ID)
}
