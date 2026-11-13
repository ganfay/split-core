package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

type mockFundUC struct {
	getInfoFn     func(ctx context.Context, req *domain.Fund) (*domain.Fund, error)
	createFundFn  func(ctx context.Context, fund *domain.Fund) (*domain.Fund, error)
	addExpenseFn  func(ctx context.Context, fundID int, id int64, desc string, cost float64) error
	getByUserFn   func(ctx context.Context, IID int64, limit, offset int) ([]domain.Fund, error)
	getMembersFn  func(ctx context.Context, fundID int) ([]domain.User, error)
	getPurchaseFn func(ctx context.Context, fundID, limit, offset int) ([]domain.Purchase, error)
}

func (m *mockFundUC) GetBalance(_ context.Context, _ int) (*domain.Settlement, error) {
	return &domain.Settlement{Debts: []domain.Debt{}}, nil
}
func (m *mockFundUC) AddExpense(ctx context.Context, fundID int, id int64, desc string, cost float64) error {
	if m.addExpenseFn != nil {
		return m.addExpenseFn(ctx, fundID, id, desc, cost)
	}
	return nil
}
func (m *mockFundUC) CreateFund(ctx context.Context, fund *domain.Fund) (*domain.Fund, error) {
	if m.createFundFn != nil {
		return m.createFundFn(ctx, fund)
	}
	return fund, nil
}
func (m *mockFundUC) DeleteFund(_ context.Context, _ int, _ int64) error { return nil }
func (m *mockFundUC) GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error) {
	if m.getInfoFn != nil {
		return m.getInfoFn(ctx, reqFund)
	}
	return &domain.Fund{ID: reqFund.ID, Name: "Trip", AuthorID: 10, CreatedAt: time.Now()}, nil
}
func (m *mockFundUC) GetFundsByUserID(ctx context.Context, IID int64, limit int, offset int) ([]domain.Fund, error) {
	if m.getByUserFn != nil {
		return m.getByUserFn(ctx, IID, limit, offset)
	}
	return []domain.Fund{}, nil
}
func (m *mockFundUC) AddMember(_ context.Context, _ int, _ int64) error        { return nil }
func (m *mockFundUC) IsMember(_ context.Context, _ int, _ int64) (bool, error) { return true, nil }
func (m *mockFundUC) GetMembers(ctx context.Context, fundID int) ([]domain.User, error) {
	if m.getMembersFn != nil {
		return m.getMembersFn(ctx, fundID)
	}
	return []domain.User{}, nil
}
func (m *mockFundUC) GetVirtualUsers(_ context.Context, _ int, _, _ int) ([]domain.User, error) {
	return []domain.User{}, nil
}
func (m *mockFundUC) RemoveUser(_ context.Context, _ int, _ int64) error { return nil }
func (m *mockFundUC) GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error) {
	if m.getPurchaseFn != nil {
		return m.getPurchaseFn(ctx, fundID, limit, offset)
	}
	return []domain.Purchase{}, nil
}
func (m *mockFundUC) CreatePurchase(_ context.Context, _ int, _ float64, _ int64, _ string) error {
	return nil
}

type mockUserUC struct {
	createVirtualFn func(ctx context.Context, firstName string) (int64, error)
	getByIIDFn      func(ctx context.Context, iID int64) (*domain.User, error)
}

func (m *mockUserUC) GetOrCreateRealUser(_ context.Context, _ *int64, _, _ string) (int64, error) {
	return 1, nil
}
func (m *mockUserUC) CreateVirtualUser(ctx context.Context, firstName string) (int64, error) {
	if m.createVirtualFn != nil {
		return m.createVirtualFn(ctx, firstName)
	}
	return 55, nil
}
func (m *mockUserUC) GetUserByIID(ctx context.Context, iID int64) (*domain.User, error) {
	if m.getByIIDFn != nil {
		return m.getByIIDFn(ctx, iID)
	}
	return &domain.User{ID: iID, FirstName: "Author"}, nil
}
func (m *mockUserUC) DeleteUser(_ context.Context, _ int64) error { return nil }

type mockStatesUC struct {
	mu      sync.Mutex
	userCtx *domain.UserContext
}

func (m *mockStatesUC) GetUserCtx(_ context.Context, _ *int64) (*domain.UserContext, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *m.userCtx
	return &cp, nil
}
func (m *mockStatesUC) SaveUserCtx(_ context.Context, _ *int64, value *domain.UserContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *value
	m.userCtx = &cp
	return nil
}
func (m *mockStatesUC) Session(_ context.Context, _ domain.Session) error { return nil }
func (m *mockStatesUC) GetSessionCtx(_ context.Context, _ string) (domain.Session, error) {
	return domain.Session{}, nil
}

func TestMenuViewFundLogsPagination(t *testing.T) {
	h := NewBotHandler(&mockFundUC{}, &mockUserUC{}, &mockStatesUC{})

	sevenPurchases := make([]domain.Purchase, 7)
	threePurchases := make([]domain.Purchase, 3)

	tests := []struct {
		name     string
		offset   int
		items    []domain.Purchase
		wantPrev bool
		wantNext bool
	}{
		{name: "first page full has next only", offset: 0, items: sevenPurchases, wantPrev: false, wantNext: true},
		{name: "second page full has prev and next", offset: 7, items: sevenPurchases, wantPrev: true, wantNext: true},
		{name: "second page partial has prev only", offset: 7, items: threePurchases, wantPrev: true, wantNext: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			markup := h.MenuViewFundLogs(tc.offset, tc.items)
			var uniques []string
			for _, row := range markup.InlineKeyboard {
				for _, btn := range row {
					uniques = append(uniques, btn.Unique)
				}
			}
			assert.Equal(t, tc.wantPrev, contains(uniques, CommandPreviousVFL))
			assert.Equal(t, tc.wantNext, contains(uniques, CommandNextVFL))
			assert.Contains(t, uniques, CommandBack)
		})
	}
}

func contains(items []string, target string) bool {
	for _, it := range items {
		if it == target {
			return true
		}
	}
	return false
}

func newTestBotServer(t *testing.T) (*tele.Bot, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var texts []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if txt, ok := body["text"].(string); ok {
			mu.Lock()
			texts = append(texts, txt)
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/answerCallbackQuery") || strings.HasSuffix(r.URL.Path, "/deleteMessage") {
			_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42,"chat":{"id":100}}}`))
	}))
	t.Cleanup(ts.Close)

	b, err := tele.NewBot(tele.Settings{
		Token:   "123:test",
		URL:     ts.URL,
		Offline: true,
	})
	require.NoError(t, err)
	return b, &texts
}

func TestTelegramSafetyAndHTMLEscaping(t *testing.T) {
	b, sentTexts := newTestBotServer(t)

	t.Run("LoggingMiddleware handles nil sender without panic", func(t *testing.T) {
		mw := LoggingMiddleware()
		ctx := b.NewContext(tele.Update{Message: &tele.Message{Text: "hello"}})
		err := mw(func(_ tele.Context) error { return nil })(ctx)
		require.NoError(t, err)
	})

	t.Run("HandleStart malformed arg without underscore does not panic", func(t *testing.T) {
		st := &mockStatesUC{userCtx: &domain.UserContext{}}
		h := NewBotHandler(&mockFundUC{}, &mockUserUC{}, st)
		ctx := b.NewContext(tele.Update{
			Message: &tele.Message{
				ID:     1,
				Sender: &tele.User{ID: 100, FirstName: "Maks"},
				Chat:   &tele.Chat{ID: 100},
				Text:   "/start invite",
			},
		})
		require.NoError(t, h.HandleStart(ctx))
	})

	t.Run("HandleBack unknown state recovers gracefully without panic", func(t *testing.T) {
		st := &mockStatesUC{userCtx: &domain.UserContext{State: domain.State(999), InternalID: 1}}
		h := NewBotHandler(&mockFundUC{}, &mockUserUC{}, st)
		ctx := b.NewContext(tele.Update{
			Callback: &tele.Callback{
				ID:     "cb1",
				Sender: &tele.User{ID: 100, FirstName: "Maks"},
				Message: &tele.Message{
					ID:   10,
					Chat: &tele.Chat{ID: 100},
				},
			},
		})
		require.NoError(t, h.HandleBack(ctx))
		assert.Equal(t, domain.StateNone, st.userCtx.State)
	})

	t.Run("OnText stores raw unescaped name in DB and escapes HTML for Telegram", func(t *testing.T) {
		var savedVirtualName string
		st := &mockStatesUC{userCtx: &domain.UserContext{
			State:        domain.StateWaitUsername,
			InternalID:   1,
			ActiveFundID: 5,
			LastMsgID:    10,
		}}
		uUC := &mockUserUC{
			createVirtualFn: func(_ context.Context, firstName string) (int64, error) {
				savedVirtualName = firstName
				return 55, nil
			},
		}
		h := NewBotHandler(&mockFundUC{}, uUC, st)
		ctx := b.NewContext(tele.Update{
			Message: &tele.Message{
				ID:     20,
				Sender: &tele.User{ID: 100, FirstName: "Maks"},
				Chat:   &tele.Chat{ID: 100},
				Text:   "Tom & <Jerry>",
			},
		})
		require.NoError(t, h.OnText(ctx))
		assert.Equal(t, "Tom & <Jerry>", savedVirtualName)
		require.NotEmpty(t, *sentTexts)
		last := (*sentTexts)[len(*sentTexts)-1]
		assert.Contains(t, last, "Tom &amp; &lt;Jerry&gt;")
	})

	t.Run("OnText allows expense descriptions longer than 30 chars", func(t *testing.T) {
		var gotDesc string
		var gotCost float64
		st := &mockStatesUC{userCtx: &domain.UserContext{
			State:        domain.StateWaitExpense,
			InternalID:   1,
			ActiveFundID: 5,
			LastMsgID:    10,
		}}
		fUC := &mockFundUC{
			addExpenseFn: func(_ context.Context, _ int, _ int64, desc string, cost float64) error {
				gotDesc = desc
				gotCost = cost
				return nil
			},
		}
		h := NewBotHandler(fUC, &mockUserUC{}, st)
		ctx := b.NewContext(tele.Update{
			Message: &tele.Message{
				ID:     21,
				Sender: &tele.User{ID: 100, FirstName: "Maks"},
				Chat:   &tele.Chat{ID: 100},
				Text:   "125.50 Taxi from Kyiv airport to hotel <VIP>",
			},
		})
		require.NoError(t, h.OnText(ctx))
		assert.Equal(t, 125.50, gotCost)
		assert.Equal(t, "Taxi from Kyiv airport to hotel <VIP>", gotDesc)
	})
}
