package v1

import (
	"context"
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

// Ping
//
// @Summary      ping
// @Tags         fund
// @Accept       json
// @Produce      json
// @Success      200
// @Failure      500
// @Router       /api/v1/ping [get]
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

// CreateFund
//
// @Summary      create fund
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param request body domain.Fund true "Only fund name must be here"
// @Success      200 {string} domain.Fund
// @Failure      400
// @Failure      500
// @Router       /api/v1/fund [post]
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

// DeleteFund
//
// @Summary      delete fund
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.Fund true "Only fund id must be here"
// @Success      204 {string} domain.Fund
// @Failure      400
// @Failure      500
// @Router       /api/v1/fund [delete]
func (s *Handler) DeleteFund(w http.ResponseWriter, r *http.Request) {
	var req domain.Fund
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Failed to decode json ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	iid := ctx.Value("id")
	if err := s.fundUC.DeleteFund(ctx, req.ID, iid.(int64)); err != nil {
		slog.Error("Failed to delete fund", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetFundsByUserIID
//
// @Summary      getfunds by useriid
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.Pagination true "Only fund id must be here"
// @Success      200 {string} []domain.Fund
// @Failure      400
// @Failure      500
// @Router       /api/v1/fund [get]
func (s *Handler) GetFundsByUserIID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	iid := ctx.Value("id")
	var req domain.Pagination
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Failed to decode json ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	funds, err := s.fundUC.GetFundsByUserID(ctx, iid.(int64), req.Limit, req.Offset)
	if err != nil {
		slog.Error("Failed to get funds", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(funds); err != nil {
		slog.Error("Failed to encode json ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// GetBalance
//
// @Summary      getbalance fund-а
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.FundRequest true "Only fund id must be here"
// @Success      200 domain.Settlement
// @Failure      400
// @Failure      500
// @Router       /api/v1/fund/balance [get]
func (s *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	var req domain.FundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("Failed to decode json ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	settlement, err := s.fundUC.GetBalance(ctx, req.ID)
	if err != nil {
		slog.Error("Failed to get fund", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(settlement); err != nil {
		slog.Error("Failed to encode json ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
