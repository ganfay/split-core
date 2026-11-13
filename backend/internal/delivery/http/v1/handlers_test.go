package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFundUC struct {
	getBalanceFn  func(ctx context.Context, fundID int) (*domain.Settlement, error)
	addExpenseFn  func(ctx context.Context, fundID int, id int64, desc string, cost float64) error
	createFundFn  func(ctx context.Context, fund *domain.Fund) (*domain.Fund, error)
	deleteFundFn  func(ctx context.Context, fundID int, IID int64) error
	getInfoFn     func(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error)
	getByUserFn   func(ctx context.Context, IID int64, limit int, offset int) ([]domain.Fund, error)
	addMemberFn   func(ctx context.Context, fundID int, IID int64) error
	isMemberFn    func(ctx context.Context, fundID int, IID int64) (bool, error)
	getMembersFn  func(ctx context.Context, fundID int) ([]domain.User, error)
	getVirtualFn  func(ctx context.Context, fundID int, offset, limit int) ([]domain.User, error)
	removeUserFn  func(ctx context.Context, fundID int, userID int64) error
	getPurchaseFn func(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error)
}

func (m *mockFundUC) GetBalance(ctx context.Context, fundID int) (*domain.Settlement, error) {
	return m.getBalanceFn(ctx, fundID)
}
func (m *mockFundUC) AddExpense(ctx context.Context, fundID int, id int64, desc string, cost float64) error {
	return m.addExpenseFn(ctx, fundID, id, desc, cost)
}
func (m *mockFundUC) CreateFund(ctx context.Context, fund *domain.Fund) (*domain.Fund, error) {
	return m.createFundFn(ctx, fund)
}
func (m *mockFundUC) DeleteFund(ctx context.Context, fundID int, IID int64) error {
	return m.deleteFundFn(ctx, fundID, IID)
}
func (m *mockFundUC) GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error) {
	return m.getInfoFn(ctx, reqFund)
}
func (m *mockFundUC) GetFundsByUserID(ctx context.Context, IID int64, limit int, offset int) ([]domain.Fund, error) {
	return m.getByUserFn(ctx, IID, limit, offset)
}
func (m *mockFundUC) AddMember(ctx context.Context, fundID int, IID int64) error {
	return m.addMemberFn(ctx, fundID, IID)
}
func (m *mockFundUC) IsMember(ctx context.Context, fundID int, IID int64) (bool, error) {
	if m.isMemberFn != nil {
		return m.isMemberFn(ctx, fundID, IID)
	}
	return true, nil
}
func (m *mockFundUC) GetMembers(ctx context.Context, fundID int) ([]domain.User, error) {
	return m.getMembersFn(ctx, fundID)
}
func (m *mockFundUC) GetVirtualUsers(ctx context.Context, fundID int, offset, limit int) ([]domain.User, error) {
	return m.getVirtualFn(ctx, fundID, offset, limit)
}
func (m *mockFundUC) RemoveUser(ctx context.Context, fundID int, userID int64) error {
	return m.removeUserFn(ctx, fundID, userID)
}
func (m *mockFundUC) GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error) {
	return m.getPurchaseFn(ctx, fundID, limit, offset)
}
func (m *mockFundUC) CreatePurchase(_ context.Context, _ int, _ float64, _ int64, _ string) error {
	return nil
}

type mockUserUC struct {
	createVirtualFn func(ctx context.Context, firstName string) (int64, error)
	getByIIDFn      func(ctx context.Context, iID int64) (*domain.User, error)
}

func (m *mockUserUC) GetOrCreateRealUser(_ context.Context, _ *int64, _ string, _ string) (int64, error) {
	return 1, nil
}
func (m *mockUserUC) CreateVirtualUser(ctx context.Context, firstName string) (int64, error) {
	return m.createVirtualFn(ctx, firstName)
}
func (m *mockUserUC) GetUserByIID(ctx context.Context, iID int64) (*domain.User, error) {
	return m.getByIIDFn(ctx, iID)
}
func (m *mockUserUC) DeleteUser(_ context.Context, _ int64) error { return nil }

type mockStateUC struct {
	sessionFn       func(ctx context.Context, s domain.Session) error
	getSessionCtxFn func(ctx context.Context, uuid string) (domain.Session, error)
}

func (m *mockStateUC) GetUserCtx(_ context.Context, _ *int64) (*domain.UserContext, error) {
	return &domain.UserContext{}, nil
}
func (m *mockStateUC) SaveUserCtx(_ context.Context, _ *int64, _ *domain.UserContext) error {
	return nil
}
func (m *mockStateUC) Session(ctx context.Context, s domain.Session) error {
	return m.sessionFn(ctx, s)
}
func (m *mockStateUC) GetSessionCtx(ctx context.Context, uuid string) (domain.Session, error) {
	return m.getSessionCtxFn(ctx, uuid)
}

func TestJWTRoundTripAndTypeSeparation(t *testing.T) {
	t.Setenv("JWT_SECRET_ACCESS", "shared-secret")
	t.Setenv("JWT_SECRET_REFRESH", "shared-secret")

	access, err := GenerateAccessJWT(42)
	require.NoError(t, err)
	refresh, err := GenerateRefreshJWT(42)
	require.NoError(t, err)

	uid, err := ParseJWTAccess(access)
	require.NoError(t, err)
	assert.Equal(t, int64(42), uid)

	uid, err = ParseJWTRefresh(refresh)
	require.NoError(t, err)
	assert.Equal(t, int64(42), uid)

	// Even if both secrets match, refresh token cannot be used as access token or vice versa.
	_, err = ParseJWTAccess(refresh)
	require.Error(t, err)
	_, err = ParseJWTRefresh(access)
	require.Error(t, err)
}

func TestMiddlewareAndAuthMiddleware(t *testing.T) {
	t.Setenv("JWT_SECRET_ACCESS", "test-access")
	validToken, err := GenerateAccessJWT(77)
	require.NoError(t, err)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantID     int64
	}{
		{name: "missing header", authHeader: "", wantStatus: http.StatusUnauthorized},
		{name: "non-bearer header", authHeader: "Basic abc", wantStatus: http.StatusUnauthorized},
		{name: "invalid jwt", authHeader: "Bearer invalid.token.here", wantStatus: http.StatusUnauthorized},
		{name: "valid bearer jwt", authHeader: "Bearer " + validToken, wantStatus: http.StatusOK, wantID: 77},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotID int64
			handler := AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
				id, ok := currentUserID(r)
				require.True(t, ok)
				gotID = id
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/user/me", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tc.wantStatus, rr.Code)
			if tc.wantStatus == http.StatusOK {
				assert.Equal(t, tc.wantID, gotID)
			}
		})
	}

	t.Run("Middleware recovers panic", func(t *testing.T) {
		h := Middleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("boom")
		}))
		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestAuthHandlers(t *testing.T) {
	t.Setenv("JWT_SECRET_ACCESS", "acc-sec")
	t.Setenv("JWT_SECRET_REFRESH", "ref-sec")

	t.Run("CreateSession", func(t *testing.T) {
		h := NewHandler(&mockFundUC{}, &mockUserUC{}, &mockStateUC{
			sessionFn: func(_ context.Context, s domain.Session) error {
				assert.Equal(t, "pending", s.Status)
				assert.NotEmpty(t, s.Uuid)
				return nil
			},
		})
		rr := httptest.NewRecorder()
		h.CreateSession(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/telegram/init", nil))
		assert.Equal(t, http.StatusOK, rr.Code)
		var resp domain.Session
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, "pending", resp.Status)
		assert.NotEmpty(t, resp.Uuid)
	})

	t.Run("CheckSession table", func(t *testing.T) {
		tests := []struct {
			name       string
			body       string
			repoErr    error
			repoStatus string
			wantCode   int
		}{
			{name: "empty uuid", body: `{"uuid":""}`, wantCode: http.StatusBadRequest},
			{name: "redis nil", body: `{"uuid":"u1"}`, repoErr: redis.Nil, wantCode: http.StatusBadRequest},
			{name: "redis down", body: `{"uuid":"u1"}`, repoErr: errors.New("conn refused"), wantCode: http.StatusInternalServerError},
			{name: "pending", body: `{"uuid":"u1"}`, repoStatus: "pending", wantCode: http.StatusOK},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				h := NewHandler(&mockFundUC{}, &mockUserUC{}, &mockStateUC{
					getSessionCtxFn: func(_ context.Context, _ string) (domain.Session, error) {
						return domain.Session{Uuid: "u1", Status: tc.repoStatus}, tc.repoErr
					},
				})
				req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/telegram/status", bytes.NewBufferString(tc.body))
				rr := httptest.NewRecorder()
				h.CheckSession(rr, req)
				assert.Equal(t, tc.wantCode, rr.Code)
			})
		}
	})

	t.Run("GenerateAllJWTTokens and GetAccessToken", func(t *testing.T) {
		h := NewHandler(&mockFundUC{}, &mockUserUC{}, &mockStateUC{
			getSessionCtxFn: func(_ context.Context, _ string) (domain.Session, error) {
				return domain.Session{Uuid: "u1", Status: "authenticated", IID: 99}, nil
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/telegram/tokens", bytes.NewBufferString(`{"uuid":"u1"}`))
		rr := httptest.NewRecorder()
		h.GenerateAllJWTTokens(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		cookies := rr.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "refresh_token", cookies[0].Name)

		req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/telegram/access", nil)
		req2.AddCookie(cookies[0])
		rr2 := httptest.NewRecorder()
		h.GetAccessToken(rr2, req2)
		require.Equal(t, http.StatusOK, rr2.Code)
		var tok domain.Tokens
		require.NoError(t, json.Unmarshal(rr2.Body.Bytes(), &tok))
		uid, err := ParseJWTAccess(tok.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, int64(99), uid)
	})
}

func authRequest(method, path, body string, userID int64) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	ctx := context.WithValue(req.Context(), userIDContextKey, userID)
	return req.WithContext(ctx)
}

func TestFundAndUserHandlers(t *testing.T) {
	tests := []struct {
		name     string
		call     func(h *Handler, rr *httptest.ResponseRecorder)
		setup    func(*mockFundUC, *mockUserUC)
		wantCode int
	}{
		{
			name: "Ping",
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.Ping(rr, httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil))
			},
			wantCode: http.StatusOK,
		},
		{
			name: "CreateFund empty name 400",
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.CreateFund(rr, authRequest(http.MethodPost, "/api/v1/fund", `{"name":"  "}`, 1))
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "CreateFund success",
			setup: func(f *mockFundUC, _ *mockUserUC) {
				f.createFundFn = func(_ context.Context, fund *domain.Fund) (*domain.Fund, error) {
					fund.ID = 10
					return fund, nil
				}
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.CreateFund(rr, authRequest(http.MethodPost, "/api/v1/fund", `{"name":"Trip"}`, 1))
			},
			wantCode: http.StatusOK,
		},
		{
			name: "SettleFund non-positive cost returns 400 (not 200)",
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.SettleFund(rr, authRequest(http.MethodPost, "/api/v1/fund/expense", `{"fund_id":1,"cost":0,"description":"Zero"}`, 1))
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "SettleFund fractional cost 0.50 succeeds",
			setup: func(f *mockFundUC, _ *mockUserUC) {
				f.addExpenseFn = func(_ context.Context, _ int, _ int64, _ string, _ float64) error { return nil }
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.SettleFund(rr, authRequest(http.MethodPost, "/api/v1/fund/expense", `{"fund_id":1,"cost":0.5,"description":"Gum"}`, 1))
			},
			wantCode: http.StatusOK,
		},
		{
			name: "SettleFund non-member 403",
			setup: func(f *mockFundUC, _ *mockUserUC) {
				f.isMemberFn = func(_ context.Context, _ int, _ int64) (bool, error) { return false, nil }
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.SettleFund(rr, authRequest(http.MethodPost, "/api/v1/fund/expense", `{"fund_id":1,"cost":10,"description":"X"}`, 1))
			},
			wantCode: http.StatusForbidden,
		},
		{
			name: "DeleteFund forbidden 403",
			setup: func(f *mockFundUC, _ *mockUserUC) {
				f.deleteFundFn = func(_ context.Context, _ int, _ int64) error { return domain.ErrForbidden }
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.DeleteFund(rr, authRequest(http.MethodDelete, "/api/v1/fund", `{"id":1}`, 1))
			},
			wantCode: http.StatusForbidden,
		},
		{
			name: "DeleteFund not found 404",
			setup: func(f *mockFundUC, _ *mockUserUC) {
				f.deleteFundFn = func(_ context.Context, _ int, _ int64) error { return domain.ErrNotFound }
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.DeleteFund(rr, authRequest(http.MethodDelete, "/api/v1/fund", `{"id":1}`, 1))
			},
			wantCode: http.StatusNotFound,
		},
		{
			name: "DeleteFund success 204",
			setup: func(f *mockFundUC, _ *mockUserUC) {
				f.deleteFundFn = func(_ context.Context, _ int, _ int64) error { return nil }
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.DeleteFund(rr, authRequest(http.MethodDelete, "/api/v1/fund", `{"id":1}`, 1))
			},
			wantCode: http.StatusNoContent,
		},
		{
			name: "JoinFund already member returns 409",
			setup: func(f *mockFundUC, _ *mockUserUC) {
				f.getInfoFn = func(_ context.Context, _ *domain.Fund) (*domain.Fund, error) {
					return &domain.Fund{ID: 1, Name: "Trip"}, nil
				}
				f.addMemberFn = func(_ context.Context, _ int, _ int64) error { return domain.ErrAlreadyMember }
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.JoinFund(rr, authRequest(http.MethodPost, "/api/v1/fund/join", `{"invite_code":"abc123"}`, 1))
			},
			wantCode: http.StatusConflict,
		},
		{
			name: "GetFundInfo not found returns 404",
			setup: func(f *mockFundUC, _ *mockUserUC) {
				f.getInfoFn = func(_ context.Context, _ *domain.Fund) (*domain.Fund, error) { return nil, domain.ErrNotFound }
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.GetFundInfo(rr, authRequest(http.MethodPost, "/api/v1/fund/info", `{"id":999}`, 1))
			},
			wantCode: http.StatusNotFound,
		},
		{
			name: "GetBalance success",
			setup: func(f *mockFundUC, _ *mockUserUC) {
				f.getBalanceFn = func(_ context.Context, _ int) (*domain.Settlement, error) {
					return &domain.Settlement{TotalAmount: 100, Average: 50, Debts: []domain.Debt{}}, nil
				}
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.GetBalance(rr, authRequest(http.MethodPost, "/api/v1/fund/balance", `{"id":1}`, 1))
			},
			wantCode: http.StatusOK,
		},
		{
			name: "GetFundsByUserIID empty GET body uses defaults",
			setup: func(f *mockFundUC, _ *mockUserUC) {
				f.getByUserFn = func(_ context.Context, _ int64, limit, offset int) ([]domain.Fund, error) {
					assert.Equal(t, 20, limit)
					assert.Equal(t, 0, offset)
					return []domain.Fund{}, nil
				}
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.GetFundsByUserIID(rr, authRequest(http.MethodGet, "/api/v1/fund", "", 1))
			},
			wantCode: http.StatusOK,
		},
		{
			name: "GetMe not found 404",
			setup: func(_ *mockFundUC, u *mockUserUC) {
				u.getByIIDFn = func(_ context.Context, _ int64) (*domain.User, error) { return nil, domain.ErrNotFound }
			},
			call: func(h *Handler, rr *httptest.ResponseRecorder) {
				h.GetMe(rr, authRequest(http.MethodGet, "/api/v1/user/me", "", 1))
			},
			wantCode: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &mockFundUC{}
			u := &mockUserUC{}
			if tc.setup != nil {
				tc.setup(f, u)
			}
			h := NewHandler(f, u, &mockStateUC{})
			rr := httptest.NewRecorder()
			tc.call(h, rr)
			assert.Equal(t, tc.wantCode, rr.Code)
		})
	}
}
