package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserRepo struct {
	getOrCreateFn func(ctx context.Context, tgID *int64, username, firstName string) (int64, error)
	createVirtFn  func(ctx context.Context, firstName string) (int64, error)
	getByIIDFn    func(ctx context.Context, iID int64) (*domain.User, error)
	deleteUserFn  func(ctx context.Context, iID int64) error
}

func (m *mockUserRepo) GetOrCreateRealUser(ctx context.Context, tgID *int64, username, firstName string) (int64, error) {
	return m.getOrCreateFn(ctx, tgID, username, firstName)
}
func (m *mockUserRepo) CreateVirtualUser(ctx context.Context, firstName string) (int64, error) {
	return m.createVirtFn(ctx, firstName)
}
func (m *mockUserRepo) GetUserByIID(ctx context.Context, iID int64) (*domain.User, error) {
	return m.getByIIDFn(ctx, iID)
}
func (m *mockUserRepo) DeleteUser(ctx context.Context, iID int64) error {
	return m.deleteUserFn(ctx, iID)
}

type mockRedisRepo struct {
	getUserCtxFn    func(ctx context.Context, userID *int64) (*domain.UserContext, error)
	saveUserCtxFn   func(ctx context.Context, userID *int64, value *domain.UserContext) error
	sessionFn       func(ctx context.Context, s domain.Session) error
	getSessionCtxFn func(ctx context.Context, uuid string) (domain.Session, error)
}

func (m *mockRedisRepo) GetUserCtx(ctx context.Context, userID *int64) (*domain.UserContext, error) {
	return m.getUserCtxFn(ctx, userID)
}
func (m *mockRedisRepo) SaveUserCtx(ctx context.Context, userID *int64, value *domain.UserContext) error {
	return m.saveUserCtxFn(ctx, userID, value)
}
func (m *mockRedisRepo) Session(ctx context.Context, s domain.Session) error {
	return m.sessionFn(ctx, s)
}
func (m *mockRedisRepo) GetSessionCtx(ctx context.Context, uuid string) (domain.Session, error) {
	return m.getSessionCtxFn(ctx, uuid)
}

func TestUserUsecase(t *testing.T) {
	tgID := int64(123456)
	repoErr := errors.New("db error")

	tests := []struct {
		name string
		run  func(t *testing.T, uc *UserUsecase)
	}{
		{
			name: "GetOrCreateRealUser success",
			run: func(t *testing.T, uc *UserUsecase) {
				id, err := uc.GetOrCreateRealUser(context.Background(), &tgID, "ganfay", "Maks")
				require.NoError(t, err)
				assert.Equal(t, int64(7), id)
			},
		},
		{
			name: "CreateVirtualUser success",
			run: func(t *testing.T, uc *UserUsecase) {
				id, err := uc.CreateVirtualUser(context.Background(), "Ghost")
				require.NoError(t, err)
				assert.Equal(t, int64(99), id)
			},
		},
		{
			name: "GetUserByIID success",
			run: func(t *testing.T, uc *UserUsecase) {
				u, err := uc.GetUserByIID(context.Background(), 7)
				require.NoError(t, err)
				assert.Equal(t, "Maks", u.FirstName)
			},
		},
		{
			name: "DeleteUser error propagates",
			run: func(t *testing.T, uc *UserUsecase) {
				err := uc.DeleteUser(context.Background(), 7)
				require.ErrorIs(t, err, repoErr)
			},
		},
	}

	repo := &mockUserRepo{
		getOrCreateFn: func(_ context.Context, id *int64, username, firstName string) (int64, error) {
			assert.Equal(t, tgID, *id)
			assert.Equal(t, "ganfay", username)
			assert.Equal(t, "Maks", firstName)
			return 7, nil
		},
		createVirtFn: func(_ context.Context, firstName string) (int64, error) {
			assert.Equal(t, "Ghost", firstName)
			return 99, nil
		},
		getByIIDFn: func(_ context.Context, iID int64) (*domain.User, error) {
			return &domain.User{ID: iID, FirstName: "Maks"}, nil
		},
		deleteUserFn: func(_ context.Context, _ int64) error {
			return repoErr
		},
	}
	uc := NewUserUsecase(repo)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, uc)
		})
	}
}

func TestStatesUsecase(t *testing.T) {
	tgID := int64(555)
	redisErr := errors.New("redis unavailable")

	tests := []struct {
		name string
		run  func(t *testing.T, uc *StatesUsecase)
	}{
		{
			name: "GetUserCtx",
			run: func(t *testing.T, uc *StatesUsecase) {
				uCtx, err := uc.GetUserCtx(context.Background(), &tgID)
				require.NoError(t, err)
				assert.Equal(t, domain.StateViewFund, uCtx.State)
				assert.Equal(t, 4, uCtx.ActiveFundID)
			},
		},
		{
			name: "SaveUserCtx",
			run: func(t *testing.T, uc *StatesUsecase) {
				err := uc.SaveUserCtx(context.Background(), &tgID, &domain.UserContext{State: domain.StateNone})
				require.NoError(t, err)
			},
		},
		{
			name: "Session",
			run: func(t *testing.T, uc *StatesUsecase) {
				err := uc.Session(context.Background(), domain.Session{Uuid: "abc", Status: "pending"})
				require.NoError(t, err)
			},
		},
		{
			name: "GetSessionCtx error",
			run: func(t *testing.T, uc *StatesUsecase) {
				_, err := uc.GetSessionCtx(context.Background(), "missing")
				require.ErrorIs(t, err, redisErr)
			},
		},
	}

	repo := &mockRedisRepo{
		getUserCtxFn: func(_ context.Context, _ *int64) (*domain.UserContext, error) {
			return &domain.UserContext{State: domain.StateViewFund, ActiveFundID: 4}, nil
		},
		saveUserCtxFn: func(_ context.Context, _ *int64, _ *domain.UserContext) error {
			return nil
		},
		sessionFn: func(_ context.Context, s domain.Session) error {
			assert.Equal(t, "abc", s.Uuid)
			return nil
		},
		getSessionCtxFn: func(_ context.Context, _ string) (domain.Session, error) {
			return domain.Session{}, redisErr
		},
	}
	uc := NewStateUsecase(repo)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, uc)
		})
	}
}
