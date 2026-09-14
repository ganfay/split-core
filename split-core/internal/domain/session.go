package domain

type PingResponse struct {
	Pong bool `json:"pong"`
}

type SessionCheckRequest struct {
	Uuid string `json:"uuid"`
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
