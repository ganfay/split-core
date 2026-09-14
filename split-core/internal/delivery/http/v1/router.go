package v1

import (
	"net/http"
)

func (s *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/ping", s.Ping)
	mux.HandleFunc("POST /api/v1/auth/telegram/init", s.CreateSession)
	mux.HandleFunc("GET /api/v1/auth/telegram/status", s.CheckSession)
	mux.HandleFunc("POST /api/v1/auth/tokens", s.GenerateAllJWTTokens)
}
