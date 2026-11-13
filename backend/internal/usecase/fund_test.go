package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFundRepo struct {
	mu             sync.Mutex
	createFundFn   func(ctx context.Context, fund *domain.Fund) (*domain.Fund, error)
	deleteFundFn   func(ctx context.Context, fundID int) error
	getInfoFn      func(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error)
	getByUserIDFn  func(ctx context.Context, userID int64, limit int, offset int) ([]domain.Fund, error)
	getMembersFn   func(ctx context.Context, fundID int) ([]domain.User, error)
	getVirtualFn   func(ctx context.Context, fundID int, offset, limit int) ([]domain.User, error)
	removeUserFn   func(ctx context.Context, fundID int, userID int64) error
	addMemberFn    func(ctx context.Context, fundID int, userID int64) error
	isMemberFn     func(ctx context.Context, fundID int, IID int64) (bool, error)
	deleteCalls    int
	lastDeletedID  int
	addMemberCalls int
}

func (m *mockFundRepo) CreateFund(ctx context.Context, fund *domain.Fund) (*domain.Fund, error) {
	if m.createFundFn != nil {
		return m.createFundFn(ctx, fund)
	}
	return fund, nil
}

func (m *mockFundRepo) DeleteFund(ctx context.Context, fundID int) error {
	m.mu.Lock()
	m.deleteCalls++
	m.lastDeletedID = fundID
	m.mu.Unlock()
	if m.deleteFundFn != nil {
		return m.deleteFundFn(ctx, fundID)
	}
	return nil
}

func (m *mockFundRepo) GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error) {
	if m.getInfoFn != nil {
		return m.getInfoFn(ctx, reqFund)
	}
	return nil, domain.ErrNotFound
}

func (m *mockFundRepo) GetByUserID(ctx context.Context, userID int64, limit int, offset int) ([]domain.Fund, error) {
	if m.getByUserIDFn != nil {
		return m.getByUserIDFn(ctx, userID, limit, offset)
	}
	return []domain.Fund{}, nil
}

func (m *mockFundRepo) GetMembers(ctx context.Context, fundID int) ([]domain.User, error) {
	if m.getMembersFn != nil {
		return m.getMembersFn(ctx, fundID)
	}
	return []domain.User{}, nil
}

func (m *mockFundRepo) GetVirtualUsers(ctx context.Context, fundID int, offset, limit int) ([]domain.User, error) {
	if m.getVirtualFn != nil {
		return m.getVirtualFn(ctx, fundID, offset, limit)
	}
	return []domain.User{}, nil
}

func (m *mockFundRepo) RemoveUser(ctx context.Context, fundID int, userID int64) error {
	if m.removeUserFn != nil {
		return m.removeUserFn(ctx, fundID, userID)
	}
	return nil
}

func (m *mockFundRepo) AddMember(ctx context.Context, fundID int, userID int64) error {
	m.mu.Lock()
	m.addMemberCalls++
	m.mu.Unlock()
	if m.addMemberFn != nil {
		return m.addMemberFn(ctx, fundID, userID)
	}
	return nil
}

func (m *mockFundRepo) IsMember(ctx context.Context, fundID int, IID int64) (bool, error) {
	if m.isMemberFn != nil {
		return m.isMemberFn(ctx, fundID, IID)
	}
	return false, nil
}

type recordedPurchase struct {
	fundID int
	amount float64
	iid    int64
	desc   string
}

type mockPurchaseRepo struct {
	mu           sync.Mutex
	paginationFn func(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error)
	allFn        func(ctx context.Context, fundID int) ([]domain.Purchase, error)
	createFn     func(ctx context.Context, fundID int, amount float64, IID int64, desc string) error
	created      []recordedPurchase
}

func (m *mockPurchaseRepo) GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error) {
	if m.paginationFn != nil {
		return m.paginationFn(ctx, fundID, limit, offset)
	}
	return []domain.Purchase{}, nil
}

func (m *mockPurchaseRepo) GetPurchasesByFundAll(ctx context.Context, fundID int) ([]domain.Purchase, error) {
	if m.allFn != nil {
		return m.allFn(ctx, fundID)
	}
	return []domain.Purchase{}, nil
}

func (m *mockPurchaseRepo) CreatePurchase(ctx context.Context, fundID int, amount float64, IID int64, desc string) error {
	m.mu.Lock()
	m.created = append(m.created, recordedPurchase{fundID: fundID, amount: amount, iid: IID, desc: desc})
	m.mu.Unlock()
	if m.createFn != nil {
		return m.createFn(ctx, fundID, amount, IID, desc)
	}
	return nil
}

type publishedMessage struct {
	queueName string
	body      []byte
	eventType string
}

type mockEventPublisher struct {
	mu        sync.Mutex
	publishFn func(ctx context.Context, queueName string, body []byte, eventType string) error
	messages  []publishedMessage
}

func (m *mockEventPublisher) Publish(ctx context.Context, queueName string, body []byte, eventType string) error {
	m.mu.Lock()
	m.messages = append(m.messages, publishedMessage{
		queueName: queueName,
		body:      append([]byte(nil), body...),
		eventType: eventType,
	})
	m.mu.Unlock()
	if m.publishFn != nil {
		return m.publishFn(ctx, queueName, body, eventType)
	}
	return nil
}

func (m *mockEventPublisher) Close() {}

func TestSettleUp(t *testing.T) {
	type testCase struct {
		name      string
		purchases []domain.Purchase
		members   []domain.User
		expected  *domain.Settlement
	}

	tests := []testCase{
		{
			name: "One paid for three",
			purchases: []domain.Purchase{
				{
					ID: 1, FundID: 1, Payer: domain.User{ID: 1, Username: "FirstPayer"}, Amount: 300, Description: "First payment description",
				},
			},
			members: []domain.User{
				{ID: 1, Username: "FirstUser"},
				{ID: 2, Username: "SecondUser"},
				{ID: 3, Username: "ThirdUser"},
			},
			expected: &domain.Settlement{
				TotalAmount: 300,
				Average:     100,
				Debts: []domain.Debt{
					{FromID: 2, ToID: 1, Amount: 100},
					{FromID: 3, ToID: 1, Amount: 100},
				},
			},
		},
		{
			name: "Equally",
			purchases: []domain.Purchase{
				{ID: 1, FundID: 1, Payer: domain.User{ID: 1, Username: "FirstPayer"}, Amount: 300, Description: "First payment description"},
				{ID: 2, FundID: 1, Payer: domain.User{ID: 2, Username: "SecondUser"}, Amount: 300, Description: "Second payment description"},
				{ID: 3, FundID: 1, Payer: domain.User{ID: 3, Username: "ThirdUser"}, Amount: 300, Description: "Third payment description"},
			},
			members: []domain.User{
				{ID: 1, Username: "FirstUser"},
				{ID: 2, Username: "SecondUser"},
				{ID: 3, Username: "ThirdUser"},
			},
			expected: &domain.Settlement{
				TotalAmount: 900,
				Average:     300,
				Debts:       []domain.Debt{},
			},
		},
		{
			name: "Complex float precision",
			purchases: []domain.Purchase{
				{ID: 1, FundID: 1, Payer: domain.User{ID: 1, Username: "FirstPayer"}, Amount: 123.52, Description: "First payment description"},
				{ID: 2, FundID: 1, Payer: domain.User{ID: 2, Username: "SecondUser"}, Amount: 2000, Description: "Second payment description"},
				{ID: 3, FundID: 1, Payer: domain.User{ID: 3, Username: "ThirdUser"}, Amount: 30, Description: "Third payment description"},
			},
			members: []domain.User{
				{ID: 1, Username: "FirstUser"},
				{ID: 2, Username: "SecondUser"},
				{ID: 3, Username: "ThirdUser"},
			},
			expected: &domain.Settlement{
				TotalAmount: 2153.52,
				Average:     717.84,
				Debts: []domain.Debt{
					{FromID: 1, ToID: 2, Amount: 594.32},
					{FromID: 3, ToID: 2, Amount: 687.84},
				},
			},
		},
		{
			name: "SomeTest",
			purchases: []domain.Purchase{
				{ID: 1, FundID: 1, Payer: domain.User{ID: 1, Username: "FirstPayer"}, Amount: 123.52, Description: "First payment description"},
				{ID: 2, FundID: 1, Payer: domain.User{ID: 2, Username: "SecondUser"}, Amount: 2000, Description: "Second payment description"},
				{ID: 3, FundID: 1, Payer: domain.User{ID: 3, Username: "ThirdUser"}, Amount: 30, Description: "Third payment description"},
				{ID: 4, FundID: 1, Payer: domain.User{ID: 4, Username: "ThirdUser"}, Amount: 3000, Description: "Third payment description"},
			},
			members: []domain.User{
				{ID: 1, Username: "FirstUser"},
				{ID: 2, Username: "SecondUser"},
				{ID: 3, Username: "ThirdUser"},
				{ID: 4, Username: "4th user"},
			},
			expected: &domain.Settlement{
				TotalAmount: 5153.52,
				Average:     1288.38,
				Debts: []domain.Debt{
					{FromID: 1, ToID: 2, Amount: 711.62},
					{FromID: 1, ToID: 4, Amount: 453.24},
					{FromID: 3, ToID: 4, Amount: 1258.38},
				},
			},
		},
		{
			name:      "Empty members and empty purchases avoids NaN",
			purchases: nil,
			members:   nil,
			expected: &domain.Settlement{
				TotalAmount: 0,
				Average:     0,
				Debts:       []domain.Debt{},
			},
		},
		{
			name: "Empty members with purchases avoids +Inf",
			purchases: []domain.Purchase{
				{ID: 1, FundID: 1, Payer: domain.User{ID: 1}, Amount: 50},
			},
			members: []domain.User{},
			expected: &domain.Settlement{
				TotalAmount: 50,
				Average:     0,
				Debts:       []domain.Debt{},
			},
		},
		{
			name:      "Members with no purchases",
			purchases: []domain.Purchase{},
			members: []domain.User{
				{ID: 1},
				{ID: 2},
			},
			expected: &domain.Settlement{
				TotalAmount: 0,
				Average:     0,
				Debts:       []domain.Debt{},
			},
		},
		{
			name: "Single member paid",
			purchases: []domain.Purchase{
				{ID: 1, FundID: 1, Payer: domain.User{ID: 1}, Amount: 100},
			},
			members: []domain.User{
				{ID: 1},
			},
			expected: &domain.Settlement{
				TotalAmount: 100,
				Average:     100,
				Debts:       []domain.Debt{},
			},
		},
		{
			name: "Duplicate members deduplicated",
			purchases: []domain.Purchase{
				{ID: 1, FundID: 1, Payer: domain.User{ID: 1}, Amount: 100},
			},
			members: []domain.User{
				{ID: 1},
				{ID: 1},
				{ID: 2},
			},
			expected: &domain.Settlement{
				TotalAmount: 100,
				Average:     50,
				Debts: []domain.Debt{
					{FromID: 2, ToID: 1, Amount: 50},
				},
			},
		},
		{
			name: "Repeating decimal 100 split by 3",
			purchases: []domain.Purchase{
				{ID: 1, FundID: 1, Payer: domain.User{ID: 1}, Amount: 100},
			},
			members: []domain.User{
				{ID: 1},
				{ID: 2},
				{ID: 3},
			},
			expected: &domain.Settlement{
				TotalAmount: 100,
				Average:     100.0 / 3.0,
				Debts: []domain.Debt{
					{FromID: 2, ToID: 1, Amount: 33.33},
					{FromID: 3, ToID: 1, Amount: 33.33},
				},
			},
		},
		{
			name: "Sub-cent balance difference ignored",
			purchases: []domain.Purchase{
				{ID: 1, FundID: 1, Payer: domain.User{ID: 1}, Amount: 10.0078125},
				{ID: 2, FundID: 1, Payer: domain.User{ID: 2}, Amount: 10.0},
			},
			members: []domain.User{
				{ID: 1},
				{ID: 2},
			},
			expected: &domain.Settlement{
				TotalAmount: 20.0078125,
				Average:     10.00390625,
				Debts:       []domain.Debt{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			settlements := calculateSettlements(tc.purchases, tc.members)
			require.NotNil(t, settlements)
			assert.False(t, math.IsNaN(settlements.Average))
			assert.False(t, math.IsInf(settlements.Average, 0))
			assert.Equal(t, tc.expected, settlements)
		})
	}
}

func TestFundUsecase_GetBalance(t *testing.T) {
	repoErr := errors.New("db error")

	tests := []struct {
		name       string
		fundID     int
		setupRepos func(*mockFundRepo, *mockPurchaseRepo)
		wantErr    error
		wantTotal  float64
		wantDebts  int
	}{
		{
			name:   "success",
			fundID: 1,
			setupRepos: func(fr *mockFundRepo, pr *mockPurchaseRepo) {
				pr.allFn = func(_ context.Context, _ int) ([]domain.Purchase, error) {
					return []domain.Purchase{{ID: 1, FundID: 1, Payer: domain.User{ID: 1}, Amount: 200}}, nil
				}
				fr.getMembersFn = func(_ context.Context, _ int) ([]domain.User, error) {
					return []domain.User{{ID: 1}, {ID: 2}}, nil
				}
			},
			wantTotal: 200,
			wantDebts: 1,
		},
		{
			name:       "invalid fundID",
			fundID:     0,
			setupRepos: func(_ *mockFundRepo, _ *mockPurchaseRepo) {},
			wantErr:    domain.ErrInvalidInput,
		},
		{
			name:   "purchase repository error",
			fundID: 1,
			setupRepos: func(_ *mockFundRepo, pr *mockPurchaseRepo) {
				pr.allFn = func(_ context.Context, _ int) ([]domain.Purchase, error) {
					return nil, repoErr
				}
			},
			wantErr: repoErr,
		},
		{
			name:   "members repository error",
			fundID: 1,
			setupRepos: func(fr *mockFundRepo, pr *mockPurchaseRepo) {
				pr.allFn = func(_ context.Context, _ int) ([]domain.Purchase, error) {
					return []domain.Purchase{}, nil
				}
				fr.getMembersFn = func(_ context.Context, _ int) ([]domain.User, error) {
					return nil, repoErr
				}
			},
			wantErr: repoErr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fr := &mockFundRepo{}
			pr := &mockPurchaseRepo{}
			tc.setupRepos(fr, pr)
			uc := NewFundUsecase(fr, pr, &mockEventPublisher{})

			got, err := uc.GetBalance(context.Background(), tc.fundID)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tc.wantTotal, got.TotalAmount)
			assert.Len(t, got.Debts, tc.wantDebts)
		})
	}
}

func TestFundUsecase_AddExpense(t *testing.T) {
	dbErr := errors.New("postgres down")
	longUkrainianDesc := strings.Repeat("Ї", 220) // 220 multi-byte runes

	tests := []struct {
		name           string
		fundID         int
		userID         int64
		desc           string
		cost           float64
		nilPublisher   bool
		setupMocks     func(*mockFundRepo, *mockPurchaseRepo, *mockEventPublisher)
		wantErr        error
		wantCreated    int
		wantPublished  int
		checkCreatedFn func(*testing.T, recordedPurchase)
	}{
		{
			name:   "success",
			fundID: 10,
			userID: 42,
			desc:   "Taxi to hotel",
			cost:   150.5,
			setupMocks: func(fr *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {
				fr.isMemberFn = func(_ context.Context, _ int, _ int64) (bool, error) { return true, nil }
			},
			wantCreated:   1,
			wantPublished: 1,
			checkCreatedFn: func(t *testing.T, rec recordedPurchase) {
				assert.Equal(t, 10, rec.fundID)
				assert.Equal(t, int64(42), rec.iid)
				assert.Equal(t, 150.5, rec.amount)
				assert.Equal(t, "Taxi to hotel", rec.desc)
			},
		},
		{
			name:   "non-member returns ErrForbidden without creating purchase",
			fundID: 10,
			userID: 42,
			desc:   "Dinner",
			cost:   50,
			setupMocks: func(fr *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {
				fr.isMemberFn = func(_ context.Context, _ int, _ int64) (bool, error) { return false, nil }
			},
			wantErr:       domain.ErrForbidden,
			wantCreated:   0,
			wantPublished: 0,
		},
		{
			name:   "isMember error propagates",
			fundID: 10,
			userID: 42,
			desc:   "Dinner",
			cost:   50,
			setupMocks: func(fr *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {
				fr.isMemberFn = func(_ context.Context, _ int, _ int64) (bool, error) { return false, dbErr }
			},
			wantErr:       dbErr,
			wantCreated:   0,
			wantPublished: 0,
		},
		{
			name:       "invalid fundID",
			fundID:     0,
			userID:     42,
			desc:       "Dinner",
			cost:       50,
			setupMocks: func(_ *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {},
			wantErr:    domain.ErrInvalidInput,
		},
		{
			name:       "invalid userID",
			fundID:     10,
			userID:     0,
			desc:       "Dinner",
			cost:       50,
			setupMocks: func(_ *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {},
			wantErr:    domain.ErrInvalidInput,
		},
		{
			name:       "zero cost rejected",
			fundID:     10,
			userID:     42,
			desc:       "Dinner",
			cost:       0,
			setupMocks: func(_ *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {},
			wantErr:    domain.ErrInvalidAmount,
		},
		{
			name:       "negative cost rejected",
			fundID:     10,
			userID:     42,
			desc:       "Dinner",
			cost:       -10,
			setupMocks: func(_ *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {},
			wantErr:    domain.ErrInvalidAmount,
		},
		{
			name:       "NaN cost rejected",
			fundID:     10,
			userID:     42,
			desc:       "Dinner",
			cost:       math.NaN(),
			setupMocks: func(_ *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {},
			wantErr:    domain.ErrInvalidAmount,
		},
		{
			name:       "+Inf cost rejected",
			fundID:     10,
			userID:     42,
			desc:       "Dinner",
			cost:       math.Inf(1),
			setupMocks: func(_ *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {},
			wantErr:    domain.ErrInvalidAmount,
		},
		{
			name:   "long UTF-8 description truncated by runes preserving valid UTF-8",
			fundID: 10,
			userID: 42,
			desc:   longUkrainianDesc,
			cost:   25,
			setupMocks: func(fr *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {
				fr.isMemberFn = func(_ context.Context, _ int, _ int64) (bool, error) { return true, nil }
			},
			wantCreated:   1,
			wantPublished: 1,
			checkCreatedFn: func(t *testing.T, rec recordedPurchase) {
				assert.True(t, utf8.ValidString(rec.desc))
				assert.Equal(t, 200, utf8.RuneCountInString(rec.desc))
				assert.True(t, strings.HasSuffix(rec.desc, "..."))
			},
		},
		{
			name:   "CreatePurchase error stops before publish",
			fundID: 10,
			userID: 42,
			desc:   "Dinner",
			cost:   50,
			setupMocks: func(fr *mockFundRepo, pr *mockPurchaseRepo, _ *mockEventPublisher) {
				fr.isMemberFn = func(_ context.Context, _ int, _ int64) (bool, error) { return true, nil }
				pr.createFn = func(_ context.Context, _ int, _ float64, _ int64, _ string) error { return dbErr }
			},
			wantErr:       dbErr,
			wantCreated:   1,
			wantPublished: 0,
		},
		{
			name:   "publish error after purchase persisted does not fail request",
			fundID: 10,
			userID: 42,
			desc:   "Dinner",
			cost:   50,
			setupMocks: func(fr *mockFundRepo, _ *mockPurchaseRepo, ep *mockEventPublisher) {
				fr.isMemberFn = func(_ context.Context, _ int, _ int64) (bool, error) { return true, nil }
				ep.publishFn = func(_ context.Context, _ string, _ []byte, _ string) error {
					return errors.New("rabbitmq offline")
				}
			},
			wantErr:       nil,
			wantCreated:   1,
			wantPublished: 1,
		},
		{
			name:         "nil publisher succeeds",
			fundID:       10,
			userID:       42,
			desc:         "Dinner",
			cost:         50,
			nilPublisher: true,
			setupMocks: func(fr *mockFundRepo, _ *mockPurchaseRepo, _ *mockEventPublisher) {
				fr.isMemberFn = func(_ context.Context, _ int, _ int64) (bool, error) { return true, nil }
			},
			wantCreated: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fr := &mockFundRepo{}
			pr := &mockPurchaseRepo{}
			ep := &mockEventPublisher{}
			tc.setupMocks(fr, pr, ep)

			var uc *FundUsecase
			if tc.nilPublisher {
				uc = NewFundUsecase(fr, pr, nil)
			} else {
				uc = NewFundUsecase(fr, pr, ep)
			}

			err := uc.AddExpense(context.Background(), tc.fundID, tc.userID, tc.desc, tc.cost)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, pr.created, tc.wantCreated)
			if tc.checkCreatedFn != nil && len(pr.created) > 0 {
				tc.checkCreatedFn(t, pr.created[0])
			}
			assert.Len(t, ep.messages, tc.wantPublished)
			if tc.wantPublished > 0 && err == nil {
				assert.Equal(t, "test", ep.messages[0].queueName)
				assert.Equal(t, "expense_created", ep.messages[0].eventType)
				var ev ExpenseCreatedEvent
				require.NoError(t, json.Unmarshal(ep.messages[0].body, &ev))
				assert.Equal(t, int64(tc.fundID), ev.FundID)
				assert.Equal(t, tc.userID, ev.CreatorID)
				assert.Equal(t, tc.cost, ev.Amount)
			}
		})
	}
}

func TestFundUsecase_DeleteFund(t *testing.T) {
	dbErr := errors.New("delete failed")

	tests := []struct {
		name        string
		fundID      int
		userID      int64
		setupRepo   func(*mockFundRepo)
		wantErr     error
		wantDeletes int
	}{
		{
			name:   "author deletes fund",
			fundID: 5,
			userID: 100,
			setupRepo: func(fr *mockFundRepo) {
				fr.getInfoFn = func(_ context.Context, _ *domain.Fund) (*domain.Fund, error) {
					return &domain.Fund{ID: 5, AuthorID: 100}, nil
				}
			},
			wantDeletes: 1,
		},
		{
			name:   "non-author forbidden",
			fundID: 5,
			userID: 200,
			setupRepo: func(fr *mockFundRepo) {
				fr.getInfoFn = func(_ context.Context, _ *domain.Fund) (*domain.Fund, error) {
					return &domain.Fund{ID: 5, AuthorID: 100}, nil
				}
			},
			wantErr:     domain.ErrForbidden,
			wantDeletes: 0,
		},
		{
			name:   "not found",
			fundID: 5,
			userID: 100,
			setupRepo: func(fr *mockFundRepo) {
				fr.getInfoFn = func(_ context.Context, _ *domain.Fund) (*domain.Fund, error) {
					return nil, domain.ErrNotFound
				}
			},
			wantErr:     domain.ErrNotFound,
			wantDeletes: 0,
		},
		{
			name:        "invalid fundID",
			fundID:      0,
			userID:      100,
			setupRepo:   func(_ *mockFundRepo) {},
			wantErr:     domain.ErrInvalidInput,
			wantDeletes: 0,
		},
		{
			name:   "repo delete error",
			fundID: 5,
			userID: 100,
			setupRepo: func(fr *mockFundRepo) {
				fr.getInfoFn = func(_ context.Context, _ *domain.Fund) (*domain.Fund, error) {
					return &domain.Fund{ID: 5, AuthorID: 100}, nil
				}
				fr.deleteFundFn = func(_ context.Context, _ int) error {
					return dbErr
				}
			},
			wantErr:     dbErr,
			wantDeletes: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fr := &mockFundRepo{}
			tc.setupRepo(fr)
			uc := NewFundUsecase(fr, &mockPurchaseRepo{}, &mockEventPublisher{})

			err := uc.DeleteFund(context.Background(), tc.fundID, tc.userID)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantDeletes, fr.deleteCalls)
		})
	}
}

func TestFundUsecase_Delegations(t *testing.T) {
	fr := &mockFundRepo{
		createFundFn: func(_ context.Context, f *domain.Fund) (*domain.Fund, error) {
			f.ID = 9
			return f, nil
		},
		getInfoFn: func(_ context.Context, req *domain.Fund) (*domain.Fund, error) {
			return &domain.Fund{ID: req.ID, Name: "Trip"}, nil
		},
		getByUserIDFn: func(_ context.Context, _ int64, limit, offset int) ([]domain.Fund, error) {
			return []domain.Fund{{ID: 1, Name: "Trip"}}, nil
		},
		getMembersFn: func(_ context.Context, _ int) ([]domain.User, error) {
			return []domain.User{{ID: 1, FirstName: "Maks"}}, nil
		},
		getVirtualFn: func(_ context.Context, _ int, _, _ int) ([]domain.User, error) {
			return []domain.User{{ID: 2, FirstName: "Ghost", IsVirtual: true}}, nil
		},
		isMemberFn: func(_ context.Context, _ int, _ int64) (bool, error) { return true, nil },
	}
	pr := &mockPurchaseRepo{
		paginationFn: func(_ context.Context, fundID, _, _ int) ([]domain.Purchase, error) {
			return []domain.Purchase{{ID: 1, FundID: fundID, Amount: 42}}, nil
		},
		allFn: func(_ context.Context, fundID int) ([]domain.Purchase, error) {
			return []domain.Purchase{{ID: 1, FundID: fundID, Amount: 42}}, nil
		},
	}
	uc := NewFundUsecase(fr, pr, &mockEventPublisher{})
	ctx := context.Background()

	_, err := uc.CreateFund(ctx, nil)
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	created, err := uc.CreateFund(ctx, &domain.Fund{Name: "Trip"})
	require.NoError(t, err)
	assert.Equal(t, 9, created.ID)

	info, err := uc.GetInfo(ctx, &domain.Fund{ID: 9})
	require.NoError(t, err)
	assert.Equal(t, "Trip", info.Name)

	funds, err := uc.GetFundsByUserID(ctx, 1, 10, 0)
	require.NoError(t, err)
	assert.Len(t, funds, 1)

	require.NoError(t, uc.AddMember(ctx, 9, 2))
	assert.Equal(t, 1, fr.addMemberCalls)

	isM, err := uc.IsMember(ctx, 9, 1)
	require.NoError(t, err)
	assert.True(t, isM)

	members, err := uc.GetMembers(ctx, 9)
	require.NoError(t, err)
	assert.Len(t, members, 1)

	vUsers, err := uc.GetVirtualUsers(ctx, 9, 0, 10)
	require.NoError(t, err)
	assert.Len(t, vUsers, 1)

	require.NoError(t, uc.RemoveUser(ctx, 9, 2))

	pPag, err := uc.GetPurchasesByFundPagination(ctx, 9, 10, 0)
	require.NoError(t, err)
	assert.Len(t, pPag, 1)

	pAll, err := uc.GetPurchasesByFundAll(ctx, 9)
	require.NoError(t, err)
	assert.Len(t, pAll, 1)

	require.NoError(t, uc.CreatePurchase(ctx, 9, 10.5, 1, "Coffee"))
	assert.Len(t, pr.created, 1)
}

func TestFundUsecase_ConcurrentAccess(t *testing.T) {
	fr := &mockFundRepo{
		isMemberFn: func(_ context.Context, _ int, _ int64) (bool, error) { return true, nil },
		getMembersFn: func(_ context.Context, _ int) ([]domain.User, error) {
			return []domain.User{{ID: 1}, {ID: 2}, {ID: 3}}, nil
		},
	}
	pr := &mockPurchaseRepo{
		allFn: func(_ context.Context, _ int) ([]domain.Purchase, error) {
			return []domain.Purchase{{ID: 1, FundID: 1, Payer: domain.User{ID: 1}, Amount: 90}}, nil
		},
	}
	ep := &mockEventPublisher{}
	uc := NewFundUsecase(fr, pr, ep)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			err := uc.AddExpense(context.Background(), 1, int64(idx+1), "Coffee", 12.5)
			assert.NoError(t, err)
		}(i)
		go func() {
			defer wg.Done()
			res, err := uc.GetBalance(context.Background(), 1)
			assert.NoError(t, err)
			assert.Equal(t, 90.0, res.TotalAmount)
		}()
	}
	wg.Wait()
	assert.Len(t, pr.created, 20)
	assert.Len(t, ep.messages, 20)
}
