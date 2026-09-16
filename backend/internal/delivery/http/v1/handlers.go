package v1

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/ganfay/split-core/internal/pkg/utils"
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

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode json", "err", err)
	}
}

func decodeJSON(r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		slog.Error("Failed to decode json", "err", err)
		return false
	}
	return true
}

func currentUserID(r *http.Request) (int64, bool) {
	iid, ok := r.Context().Value("id").(int64)
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

func (s *Handler) requireFundMember(r *http.Request, fundID int) (int64, bool) {
	iid, ok := currentUserID(r)
	if !ok {
		return 0, false
	}
	isMember, err := s.fundUC.IsMember(r.Context(), fundID, iid)
	if err != nil {
		slog.Error("Failed to check fund membership", "err", err, "fund_id", fundID, "iid", iid)
		return 0, false
	}
	return iid, isMember
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
	ctx := r.Context()
	iid, ok := currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var req domain.Fund
	if !decodeJSON(r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
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
	if !decodeJSON(r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	iid, ok := currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if err := s.fundUC.DeleteFund(r.Context(), req.ID, iid); err != nil {
		slog.Error("Failed to delete fund", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
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
	ctx := r.Context()
	iid, ok := currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var req domain.Pagination
	if !decodeJSON(r, &req) {
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
	ctx := r.Context()
	var req domain.FundRequest
	if !decodeJSON(r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	fund, err := s.fundUC.GetInfo(ctx, &domain.Fund{ID: req.ID, InviteCode: req.InviteCode})
	if err != nil {
		slog.Error("Failed to get fund info", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, ok := s.requireFundMember(r, fund.ID); !ok {
		w.WriteHeader(http.StatusForbidden)
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
	ctx := r.Context()
	var req domain.FundRequest
	if !decodeJSON(r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(r, req.ID); !ok {
		w.WriteHeader(http.StatusForbidden)
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
	ctx := r.Context()
	var req domain.SettleFund
	if !decodeJSON(r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	iid, ok := s.requireFundMember(r, req.FundID)
	if !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if err := s.fundUC.AddExpense(ctx, req.FundID, iid, req.Description, req.Cost); err != nil {
		slog.Error("Failed to add expense", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
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
// @Failure      500
// @Router       /api/v1/fund/join [post]
func (s *Handler) JoinFund(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	iid, ok := currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var req domain.JoinFundRequest
	if !decodeJSON(r, &req) {
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
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err = s.fundUC.AddMember(ctx, fund.ID, iid); err != nil {
		slog.Error("Failed to add member to fund", "err", err)
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
	ctx := r.Context()
	var req domain.FundRequest
	if !decodeJSON(r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(r, req.ID); !ok {
		w.WriteHeader(http.StatusForbidden)
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
	ctx := r.Context()
	var req domain.FundPaginationRequest
	if !decodeJSON(r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(r, req.FundID); !ok {
		w.WriteHeader(http.StatusForbidden)
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
	ctx := r.Context()
	var req domain.VirtualUserRequest
	if !decodeJSON(r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.FirstName = strings.TrimSpace(req.FirstName)
	if req.FirstName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(r, req.FundID); !ok {
		w.WriteHeader(http.StatusForbidden)
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
	ctx := r.Context()
	var req domain.RemoveUserRequest
	if !decodeJSON(r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(r, req.FundID); !ok {
		w.WriteHeader(http.StatusForbidden)
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
	ctx := r.Context()
	var req domain.FundPaginationRequest
	if !decodeJSON(r, &req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := s.requireFundMember(r, req.FundID); !ok {
		w.WriteHeader(http.StatusForbidden)
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
