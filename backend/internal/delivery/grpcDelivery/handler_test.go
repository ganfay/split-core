package grpcDelivery

import (
	"context"
	"testing"

	"github.com/ganfay/split-core/internal/domain"
	pb "github.com/ganfay/split-proto/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockFundUC struct {
	getInfoFn    func(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error)
	getMembersFn func(ctx context.Context, fundID int) ([]domain.User, error)
}

func (m *mockFundUC) GetBalance(_ context.Context, _ int) (*domain.Settlement, error) {
	return nil, nil
}
func (m *mockFundUC) AddExpense(_ context.Context, _ int, _ int64, _ string, _ float64) error {
	return nil
}
func (m *mockFundUC) CreateFund(_ context.Context, f *domain.Fund) (*domain.Fund, error) {
	return f, nil
}
func (m *mockFundUC) DeleteFund(_ context.Context, _ int, _ int64) error { return nil }
func (m *mockFundUC) GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error) {
	return m.getInfoFn(ctx, reqFund)
}
func (m *mockFundUC) GetFundsByUserID(_ context.Context, _ int64, _, _ int) ([]domain.Fund, error) {
	return nil, nil
}
func (m *mockFundUC) AddMember(_ context.Context, _ int, _ int64) error          { return nil }
func (m *mockFundUC) IsMember(_ context.Context, _ int, _ int64) (bool, error)   { return true, nil }
func (m *mockFundUC) GetMembers(ctx context.Context, fundID int) ([]domain.User, error) {
	return m.getMembersFn(ctx, fundID)
}
func (m *mockFundUC) GetVirtualUsers(_ context.Context, _ int, _, _ int) ([]domain.User, error) {
	return nil, nil
}
func (m *mockFundUC) RemoveUser(_ context.Context, _ int, _ int64) error { return nil }
func (m *mockFundUC) GetPurchasesByFundPagination(_ context.Context, _ int, _, _ int) ([]domain.Purchase, error) {
	return nil, nil
}
func (m *mockFundUC) CreatePurchase(_ context.Context, _ int, _ float64, _ int64, _ string) error {
	return nil
}

func TestGetNotificationTargets(t *testing.T) {
	tg1 := int64(111)
	tg2 := int64(222)
	invalidTg := int64(-1)

	tests := []struct {
		name        string
		req         *pb.GetRequest
		setup       func(*mockFundUC)
		wantCode    codes.Code
		wantTargets int
		wantCreator string
		wantFund    string
	}{
		{
			name:     "nil request",
			req:      nil,
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "non-positive fund_id",
			req:      &pb.GetRequest{FundId: 0, CreatorIid: 1},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "fund not found",
			req:  &pb.GetRequest{FundId: 9, CreatorIid: 1},
			setup: func(f *mockFundUC) {
				f.getInfoFn = func(_ context.Context, _ *domain.Fund) (*domain.Fund, error) {
					return nil, domain.ErrNotFound
				}
			},
			wantCode: codes.NotFound,
		},
		{
			name: "filters virtual, creator, nil and non-positive tg_id",
			req:  &pb.GetRequest{FundId: 3, CreatorIid: 10},
			setup: func(f *mockFundUC) {
				f.getInfoFn = func(_ context.Context, _ *domain.Fund) (*domain.Fund, error) {
					return &domain.Fund{ID: 3, Name: "Kyiv Trip"}, nil
				}
				f.getMembersFn = func(_ context.Context, _ int) ([]domain.User, error) {
					return []domain.User{
						{ID: 10, FirstName: "Maks", TgID: &tg1},
						{ID: 11, FirstName: "Virtual", IsVirtual: true},
						{ID: 12, FirstName: "NilTg", TgID: nil},
						{ID: 13, FirstName: "NegTg", TgID: &invalidTg},
						{ID: 14, FirstName: "Alex", TgID: &tg2},
					}, nil
				}
			},
			wantCode:    codes.OK,
			wantTargets: 1,
			wantCreator: "Maks",
			wantFund:    "Kyiv Trip",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := &mockFundUC{}
			if tc.setup != nil {
				tc.setup(uc)
			}
			h := NewHandler(uc, nil)

			resp, err := h.GetNotificationTargets(context.Background(), tc.req)
			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, status.Code(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantFund, resp.GetFundName())
			assert.Equal(t, tc.wantCreator, resp.GetCreatorName())
			require.Len(t, resp.GetTargets(), tc.wantTargets)
			assert.Equal(t, int64(222), resp.GetTargets()[0].GetTgId())
		})
	}
}

func TestSayPingAndNotificateTargetValidation(t *testing.T) {
	h := NewHandler(&mockFundUC{}, nil)

	pingResp, err := h.SayPing(context.Background(), &pb.PingRequest{Name: "notifier"})
	require.NoError(t, err)
	assert.Equal(t, "Ping notifier", pingResp.GetMessage())

	_, err = h.NotificateTarget(context.Background(), nil)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = h.NotificateTarget(context.Background(), &pb.NotTarget{TgId: 0, Msg: "hi"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = h.NotificateTarget(context.Background(), &pb.NotTarget{TgId: 123, Msg: "hi"})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}
