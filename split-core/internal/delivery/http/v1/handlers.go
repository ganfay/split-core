package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/google/uuid"
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

func (s *Handler) CreateSession(w http.ResponseWriter, _ *http.Request) {
	u := uuid.New()
	ctx := context.Background()
	var session domain.Session
	session.Uuid = u.String()
	session.Status = "pending"
	err := s.stateUC.Session(ctx, session)
	if err != nil {
		slog.Error("Failed to create session ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var resp domain.Session
	resp.Uuid = u.String()
	resp.Status = "pending"
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resp)
	slog.Debug("Success create session ", "uuid", resp.Uuid)
	if err != nil {
		slog.Error("[HANDLER CREATESESSION] Failed to encode json ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Handler) CheckSession(w http.ResponseWriter, r *http.Request) {
	var req domain.SessionCheckRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		slog.Error("[HANDLER CHECKSESSION] Failed to decode json ", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx := context.Background()

	session, err := s.stateUC.GetSessionCtx(ctx, req.Uuid)
	if err != nil {
		slog.Error("[HANDLER CHECKSESSION] Failed to get session ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	slog.Debug("Success check session ", "uuid", session.Uuid)
	err = json.NewEncoder(w).Encode(session)
	if err != nil {
		slog.Error("[HANDLER CHECKSESSION] Failed to encode json ", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
