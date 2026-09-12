package web

import (
	"log/slog"

	"github.com/ganfay/split-core/internal/domain"
)

type ServerHandler struct {
	fundUC  domain.FundUsecase
	userUC  domain.UserUsecase
	stateUC domain.StatesUsecase
}

func NewServerHandler(fundUC domain.FundUsecase, userUC domain.UserUsecase, stateUC domain.StatesUsecase) *ServerHandler {
	slog.Info("Creating new web-server...")
	return &ServerHandler{
		fundUC:  fundUC,
		userUC:  userUC,
		stateUC: stateUC,
	}
}
