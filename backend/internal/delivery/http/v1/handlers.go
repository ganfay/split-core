package v1

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/ganfay/split-core/internal/pkg/utils"
)

const (
	requestTimeout = 8 * time.Second
	maxBodyBytes   = 1 << 20 // 1 MB
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

func requestContext(r *http.Request) (context.Context, context.CancelFunc) {
	if _, hasDeadline := r.Context().Deadline(); hasDeadline {
		return r.Context(), func() {}
	}
	return context.WithTimeout(r.Context(), requestTimeout)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	if data == nil {
		w.WriteHeader(status)
		return
	}
	b, err := json.Marshal(data)
	if err != nil {
		slog.Error("Failed to marshal json", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(append(b, '\n')); err != nil {
		slog.Error("Failed to write response", "err", err)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil {
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		slog.Error("Failed to decode json", "err", err)
		return false
	}
	return true
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil {
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	err := json.NewDecoder(r.Body).Decode(dst)
	if errors.Is(err, io.EOF) {
		return true
	}
	if err != nil {
		slog.Error("Failed to decode json", "err", err)
		return false
	}
	return true
}

func currentUserID(r *http.Request) (int64, bool) {
	iid, ok := r.Context().Value(userIDContextKey).(int64)
	return iid, ok
}

func normalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (s *Handler) requireFundMember(ctx context.Context, w http.ResponseWriter, r *http.Request, fundID int) (int64, bool) {
	iid, ok := currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return 0, false
	}
	isMember, err := s.fundUC.IsMember(ctx, fundID, iid)
	if err != nil {
		slog.Error("Failed to check fund membership", "err", err, "fund_id", fundID, "iid", iid)
		w.WriteHeader(http.StatusInternalServerError)
		return 0, false
	}
	if !isMember {
		w.WriteHeader(http.StatusForbidden)
		return 0, false
	}
	return iid, true
}

// Ping
//
// @Summary      ping
// @Tags         fund
// @Accept       json
// @Produce      json
// @Success      200 {object} domain.PingResponse
// @Failure      500
// @Router       /api/v1/ping [get]
func (s *Handler) Ping(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, domain.PingResponse{Pong: true})
}

// CreateFund
//
// @Summary      create fund
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.Fund true "Only fund name must be here"
// @Success      200 {object} domain.Fund
// @Failure      400
// @Failure      401
// @Failure      500
// @Router       /api/v1/fund [post]
func (s *Handler) CreateFund(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	iid, ok := currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var req domain.Fund
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.InviteCode = strings.TrimSpace(req.InviteCode)
	if req.InviteCode == "" {
		req.InviteCode = utils.GenerateInviteCode(6)
	}
	req.AuthorID = iid

	fund, err := s.fundUC.CreateFund(ctx, &req)
	if err != nil {
		slog.Error("Failed to create fund", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, fund)
}

// DeleteFund
//
// @Summary      delete fund
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.Fund true "Only fund id must be here"
// @Success      204
// @Failure      400
// @Failure      401
// @Failure      500
// @Router       /api/v1/fund [delete]
func (s *Handler) DeleteFund(w http.ResponseWriter, r *http.Request) {
	var req domain.Fund
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.ID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	iid, ok := currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	ctx, cancel := requestContext(r)
	defer cancel()

	if err := s.fundUC.DeleteFund(ctx, req.ID, iid); err != nil {
		slog.Error("Failed to delete fund", "err", err)
		switch {
		case errors.Is(err, domain.ErrForbidden):
			w.WriteHeader(http.StatusForbidden)
		case errors.Is(err, domain.ErrNotFound):
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetFundsByUserIID
//
// @Summary      get funds by current user iid
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.Pagination true "Pagination"
// @Success      200 {array} domain.Fund
// @Failure      400
// @Failure      401
// @Failure      500
// @Router       /api/v1/fund [get]
// @Router       /api/v1/fund/list [post]
func (s *Handler) GetFundsByUserIID(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	iid, ok := currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var req domain.Pagination
	if !decodeOptionalJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Limit, req.Offset = normalizePagination(req.Limit, req.Offset)

	funds, err := s.fundUC.GetFundsByUserID(ctx, iid, req.Limit, req.Offset)
	if err != nil {
		slog.Error("Failed to get funds", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, funds)
}

// GetFundInfo
//
// @Summary      get fund info
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.FundRequest true "Fund id or invite code"
// @Success      200 {object} domain.Fund
// @Failure      400
// @Failure      403
// @Failure      500
// @Router       /api/v1/fund/info [get]
// @Router       /api/v1/fund/info [post]
func (s *Handler) GetFundInfo(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	var req domain.FundRequest
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.InviteCode = strings.TrimSpace(req.InviteCode)
	if req.ID <= 0 && req.InviteCode == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	fund, err := s.fundUC.GetInfo(ctx, &domain.Fund{ID: req.ID, InviteCode: req.InviteCode})
	if err != nil {
		slog.Error("Failed to get fund info", "err", err)
		if errors.Is(err, domain.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, ok := s.requireFundMember(ctx, w, r, fund.ID); !ok {
		return
	}
	writeJSON(w, http.StatusOK, fund)
}

// GetBalance
//
// @Summary      get fund settlement
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.FundRequest true "Fund id"
// @Success      200 {object} domain.Settlement
// @Failure      400
// @Failure      403
// @Failure      500
// @Router       /api/v1/fund/balance [get]
// @Router       /api/v1/fund/balance [post]
func (s *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	var req domain.FundRequest
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.ID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(ctx, w, r, req.ID); !ok {
		return
	}
	settlement, err := s.fundUC.GetBalance(ctx, req.ID)
	if err != nil {
		slog.Error("Failed to get fund balance", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, settlement)
}

// SettleFund
//
// @Summary      add expense to fund
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.SettleFund true "Expense data"
// @Success      200
// @Failure      400
// @Failure      403
// @Failure      500
// @Router       /api/v1/fund/expense [post]
func (s *Handler) SettleFund(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	var req domain.SettleFund
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.FundID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	iid, ok := s.requireFundMember(ctx, w, r, req.FundID)
	if !ok {
		return
	}
	if math.IsNaN(req.Cost) || math.IsInf(req.Cost, 0) || req.Cost <= 0 {
		writeJSON(w, http.StatusBadRequest, domain.ResponseError{Error: "cost must be greater than 0"})
		return
	}
	if err := s.fundUC.AddExpense(ctx, req.FundID, iid, strings.TrimSpace(req.Description), req.Cost); err != nil {
		slog.Error("Failed to add expense", "err", err)
		switch {
		case errors.Is(err, domain.ErrInvalidAmount), errors.Is(err, domain.ErrInvalidInput):
			writeJSON(w, http.StatusBadRequest, domain.ResponseError{Error: err.Error()})
		case errors.Is(err, domain.ErrForbidden):
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

// JoinFund
//
// @Summary      join fund by invite code
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.JoinFundRequest true "Invite code"
// @Success      200 {object} domain.Fund
// @Failure      400
// @Failure      401
// @Failure      409
// @Failure      500
// @Router       /api/v1/fund/join [post]
func (s *Handler) JoinFund(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	iid, ok := currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var req domain.JoinFundRequest
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.InviteCode = strings.TrimSpace(req.InviteCode)
	if req.InviteCode == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	fund, err := s.fundUC.GetInfo(ctx, &domain.Fund{InviteCode: req.InviteCode})
	if err != nil {
		slog.Error("Failed to get fund by invite code", "err", err)
		if errors.Is(err, domain.ErrNotFound) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err = s.fundUC.AddMember(ctx, fund.ID, iid); err != nil {
		slog.Error("Failed to add member to fund", "err", err)
		if errors.Is(err, domain.ErrAlreadyMember) {
			writeJSON(w, http.StatusConflict, domain.ResponseError{Error: "already a member of this fund"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, fund)
}

// GetMembers
//
// @Summary      get fund members
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.FundRequest true "Fund id"
// @Success      200 {array} domain.User
// @Failure      400
// @Failure      403
// @Failure      500
// @Router       /api/v1/fund/members [get]
// @Router       /api/v1/fund/members [post]
func (s *Handler) GetMembers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	var req domain.FundRequest
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.ID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(ctx, w, r, req.ID); !ok {
		return
	}
	members, err := s.fundUC.GetMembers(ctx, req.ID)
	if err != nil {
		slog.Error("Failed to get members", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

// GetVirtualUsers
//
// @Summary      get virtual fund members
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.FundPaginationRequest true "Fund id and pagination"
// @Success      200 {array} domain.User
// @Failure      400
// @Failure      403
// @Failure      500
// @Router       /api/v1/fund/virtual-users [get]
// @Router       /api/v1/fund/virtual-users/list [post]
func (s *Handler) GetVirtualUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	var req domain.FundPaginationRequest
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.FundID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(ctx, w, r, req.FundID); !ok {
		return
	}
	req.Limit, req.Offset = normalizePagination(req.Limit, req.Offset)
	users, err := s.fundUC.GetVirtualUsers(ctx, req.FundID, req.Offset, req.Limit)
	if err != nil {
		slog.Error("Failed to get virtual users", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// AddVirtualUser
//
// @Summary      create and add virtual user to fund
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.VirtualUserRequest true "Virtual user data"
// @Success      200 {object} domain.User
// @Failure      400
// @Failure      403
// @Failure      500
// @Router       /api/v1/fund/virtual-users [post]
func (s *Handler) AddVirtualUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	var req domain.VirtualUserRequest
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.FirstName = strings.TrimSpace(req.FirstName)
	if req.FundID <= 0 || req.FirstName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(ctx, w, r, req.FundID); !ok {
		return
	}
	userID, err := s.userUC.CreateVirtualUser(ctx, req.FirstName)
	if err != nil {
		slog.Error("Failed to create virtual user", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err = s.fundUC.AddMember(ctx, req.FundID, userID); err != nil {
		slog.Error("Failed to add virtual user to fund", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, domain.User{
		ID:        userID,
		FirstName: req.FirstName,
		IsVirtual: true,
	})
}

// RemoveUser
//
// @Summary      remove user from fund
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.RemoveUserRequest true "Fund id and user id"
// @Success      204
// @Failure      400
// @Failure      403
// @Failure      500
// @Router       /api/v1/fund/member [delete]
func (s *Handler) RemoveUser(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	var req domain.RemoveUserRequest
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.FundID <= 0 || req.UserID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(ctx, w, r, req.FundID); !ok {
		return
	}
	if err := s.fundUC.RemoveUser(ctx, req.FundID, req.UserID); err != nil {
		slog.Error("Failed to remove user from fund", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetPurchases
//
// @Summary      get fund purchase history
// @Tags         fund
// @Accept       json
// @Produce      json
// @Param        request body domain.FundPaginationRequest true "Fund id and pagination"
// @Success      200 {array} domain.Purchase
// @Failure      400
// @Failure      403
// @Failure      500
// @Router       /api/v1/fund/purchases [get]
// @Router       /api/v1/fund/purchases [post]
func (s *Handler) GetPurchases(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	var req domain.FundPaginationRequest
	if !decodeJSON(w, r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.FundID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(ctx, w, r, req.FundID); !ok {
		return
	}
	req.Limit, req.Offset = normalizePagination(req.Limit, req.Offset)
	purchases, err := s.fundUC.GetPurchasesByFundPagination(ctx, req.FundID, req.Limit, req.Offset)
	if err != nil {
		slog.Error("Failed to get purchases", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, purchases)
}

// GetMe
//
// @Summary      Get current authenticated user info
// @Tags         user
// @Produce      json
// @Success      200 {object} domain.User
// @Failure      401
// @Failure      404
// @Router       /api/v1/user/me [get]
// @Router       /api/v1/user/me [post]
func (s *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r)
	defer cancel()

	iid, ok := currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	user, err := s.userUC.GetUserByIID(ctx, iid)
	if err != nil {
		slog.Error("Failed to get user by iid", "err", err, "iid", iid)
		if errors.Is(err, domain.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, user)
}
