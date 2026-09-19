package domain

type PingResponse struct {
	Pong bool `json:"pong"`
}

type SessionCheckRequest struct {
	Uuid string `json:"uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type Session struct {
	Uuid         string `json:"uuid"`
	Status       string `json:"status"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Firstname    string `json:"firstname"`
	Username     string `json:"username"`
	IID          int64  `json:"iid"`
	TgID         int64  `json:"tg_id"`
}

type Tokens struct {
	AccessToken string `json:"access_token"`
}
