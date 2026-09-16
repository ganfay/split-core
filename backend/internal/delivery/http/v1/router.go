package v1

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"

	_ "github.com/ganfay/split-core/docs"
)

func (s *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/ping", s.Ping)
	mux.Handle("/api/v1/swagger/", httpSwagger.WrapHandler)
	// auth
	mux.HandleFunc("POST /api/v1/auth/telegram/init", s.CreateSession)
	mux.HandleFunc("GET /api/v1/auth/telegram/status", s.CheckSession)
	mux.HandleFunc("POST /api/v1/auth/telegram/tokens", s.GenerateAllJWTTokens)
	mux.HandleFunc("GET /api/v1/auth/telegram/access", s.GetAccessToken)
	// fund (auth)
	mux.HandleFunc("POST /api/v1/fund", AuthMiddleware(s.CreateFund))
	mux.HandleFunc("DELETE /api/v1/fund", AuthMiddleware(s.DeleteFund))
	mux.HandleFunc("GET /api/v1/fund", AuthMiddleware(s.GetFundsByUserIID))
	mux.HandleFunc("GET /api/v1/fund/balance", AuthMiddleware(s.GetBalance))

}
