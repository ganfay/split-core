package web

import "net/http"

func (s *ServerHandler) SetupRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", s.Ping)
	return Middleware(mux)
}
