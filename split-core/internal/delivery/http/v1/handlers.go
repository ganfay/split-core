package v1

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ganfay/split-core/internal/domain"
)

type Handler struct {
	fundUC  domain.FundUsecase
	userUC  domain.UserUsecase
	stateUC domain.StatesUsecase
}

func NewHandler(fundUC domain.FundUsecase, userUC domain.UserUsecase, stateUC domain.StatesUsecase) *Handler {
	slog.Info("Creating new http-server...")
	return &Handler{
		fundUC:  fundUC,
		userUC:  userUC,
		stateUC: stateUC,
	}
}

func (s *Handler) Ping(w http.ResponseWriter, _ *http.Request) {
	req := domain.PingResponse{
		Pong: true,
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(req)
	if err != nil {
		slog.Error("Failed to encode json ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Handler) CreateFund(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	iid := ctx.Value("id")
	var req domain.Fund
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Failed to decode json ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
	}
	req.AuthorID = iid.(int64)
	fund, err := s.fundUC.CreateFund(ctx, &req)
	if err != nil {
		slog.Error("Failed to create fund", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(fund)
	if err != nil {
		slog.Error("Failed to encode json ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
