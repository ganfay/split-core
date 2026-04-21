package domain

import (
	"fmt"
	"time"
)

type User struct {
	TgID      int64
	Username  string
	FirstName string
	CreatedAt time.Time
}

func (u User) GetDisplayName() string {
	if u.Username != "" {
		return "@" + u.Username
	}
	if u.FirstName != "" && u.FirstName != "." {
		return u.FirstName
	}
	return fmt.Sprintf("User_%d", u.TgID%10000)
}
