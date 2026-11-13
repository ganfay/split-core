package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"time"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/ganfay/split-core/internal/repository"
)

const publishEventTimeout = 5 * time.Second

type FundUsecase struct {
	fundRepository     repository.FundRepository
	purchaseRepository repository.PurchaseRepository
	eventPublisher     repository.EventPublisher
}

type ExpenseCreatedEvent struct {
	FundID    int64   `json:"fund_id"`
	CreatorID int64   `json:"creator_id"`
	Amount    float64 `json:"amount"`
}

func NewFundUsecase(fr repository.FundRepository, pr repository.PurchaseRepository, ep repository.EventPublisher) *FundUsecase {
	return &FundUsecase{fundRepository: fr, purchaseRepository: pr, eventPublisher: ep}
}

func (u *FundUsecase) GetBalance(ctx context.Context, fundID int) (*domain.Settlement, error) {
	if fundID <= 0 {
		return nil, domain.ErrInvalidInput
	}
	purchases, err := u.purchaseRepository.GetPurchasesByFundAll(ctx, fundID)
	if err != nil {
		slog.Debug("GetBalance", "fundID", fundID, "error", err)
		return nil, err
	}
	members, err := u.fundRepository.GetMembers(ctx, fundID)
	if err != nil {
		return nil, err
	}

	settlement := calculateSettlements(purchases, members)
	return settlement, nil
}

func calculateSettlements(purchases []domain.Purchase, members []domain.User) *domain.Settlement {
	totalAmount := 0.0
	m := make(map[int64]float64)
	for _, purchase := range purchases {
		totalAmount += purchase.Amount
		m[purchase.Payer.ID] += purchase.Amount
	}

	seen := make(map[int64]struct{}, len(members))
	uniqueMembers := make([]domain.User, 0, len(members))
	for _, member := range members {
		if _, exists := seen[member.ID]; !exists {
			seen[member.ID] = struct{}{}
			uniqueMembers = append(uniqueMembers, member)
		}
	}

	debts := make([]domain.Debt, 0)
	if len(uniqueMembers) == 0 {
		return &domain.Settlement{
			TotalAmount: totalAmount,
			Debts:       debts,
			Average:     0,
		}
	}

	averageAmount := totalAmount / float64(len(uniqueMembers))
	var creditors []int64
	var debtors []int64
	for _, member := range uniqueMembers {
		m[member.ID] = m[member.ID] - averageAmount

		bal := m[member.ID]
		if bal > 0.01 {
			creditors = append(creditors, member.ID)
		} else if bal < -0.01 {
			debtors = append(debtors, member.ID)
		}
	}
	// from d -> c
	for len(debtors) > 0 && len(creditors) > 0 {
		d := debtors[0]
		c := creditors[0]

		amount := min(math.Abs(m[d]), m[c])
		roundedAmount := math.Round(amount*100) / 100
		m[d] += amount
		m[c] -= amount

		debt := domain.Debt{
			FromID: d,
			ToID:   c,
			Amount: roundedAmount,
		}
		debts = append(debts, debt)

		if math.Abs(m[d]) < 0.01 {
			debtors = debtors[1:]
		}
		if math.Abs(m[c]) < 0.01 {
			creditors = creditors[1:]
		}
	}
	return &domain.Settlement{
		TotalAmount: totalAmount,
		Debts:       debts,
		Average:     averageAmount,
	}
}

func (u *FundUsecase) AddExpense(ctx context.Context, fundID int, id int64, desc string, cost float64) error {
	if fundID <= 0 || id <= 0 {
		return domain.ErrInvalidInput
	}
	if math.IsNaN(cost) || math.IsInf(cost, 0) || cost <= 0 {
		return domain.ErrInvalidAmount
	}
	isMember, err := u.fundRepository.IsMember(ctx, fundID, id)
	if err != nil {
		return err
	}
	if !isMember {
		return domain.ErrForbidden
	}

	runes := []rune(desc)
	if len(runes) > 200 {
		desc = string(runes[:197]) + "..."
	}

	err = u.purchaseRepository.CreatePurchase(ctx, fundID, cost, id, desc)
	if err != nil {
		return err
	}

	if u.eventPublisher == nil {
		return nil
	}

	event := ExpenseCreatedEvent{
		CreatorID: id,
		FundID:    int64(fundID),
		Amount:    cost,
	}

	body, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal event", "err", err)
		return nil
	}

	// Expense is already persisted; keep event publishing bounded and non-fatal to avoid duplicate retries.
	pubCtx, cancel := context.WithTimeout(ctx, publishEventTimeout)
	defer cancel()
	if err = u.eventPublisher.Publish(pubCtx, "test", body, "expense_created"); err != nil {
		slog.Error("failed to publish expense_created event", "err", err, "fund_id", fundID, "creator_id", id)
	}
	return nil
}

func (u *FundUsecase) CreateFund(ctx context.Context, fund *domain.Fund) (*domain.Fund, error) {
	if fund == nil {
		return nil, domain.ErrInvalidInput
	}
	return u.fundRepository.CreateFund(ctx, fund)
}

func (u *FundUsecase) DeleteFund(ctx context.Context, fundID int, IID int64) error {
	if fundID <= 0 {
		return domain.ErrInvalidInput
	}
	fund := &domain.Fund{ID: fundID}
	fund, err := u.fundRepository.GetInfo(ctx, fund)
	if err != nil {
		return err
	}
	if fund.AuthorID != IID {
		return domain.ErrForbidden
	}
	return u.fundRepository.DeleteFund(ctx, fundID)
}

func (u *FundUsecase) GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error) {
	return u.fundRepository.GetInfo(ctx, reqFund)
}

func (u *FundUsecase) GetFundsByUserID(ctx context.Context, userID int64, limit int, offset int) ([]domain.Fund, error) {
	return u.fundRepository.GetByUserID(ctx, userID, limit, offset)
}

func (u *FundUsecase) AddMember(ctx context.Context, fundID int, userID int64) error {
	return u.fundRepository.AddMember(ctx, fundID, userID)
}

func (u *FundUsecase) IsMember(ctx context.Context, fundID int, userID int64) (bool, error) {
	return u.fundRepository.IsMember(ctx, fundID, userID)
}

func (u *FundUsecase) GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error) {
	return u.purchaseRepository.GetPurchasesByFundPagination(ctx, fundID, limit, offset)
}

func (u *FundUsecase) CreatePurchase(ctx context.Context, fundID int, amount float64, IID int64, desc string) error {
	return u.purchaseRepository.CreatePurchase(ctx, fundID, amount, IID, desc)
}

func (u *FundUsecase) GetMembers(ctx context.Context, fundID int) ([]domain.User, error) {
	return u.fundRepository.GetMembers(ctx, fundID)
}

func (u *FundUsecase) GetPurchasesByFundAll(ctx context.Context, fundID int) ([]domain.Purchase, error) {
	return u.purchaseRepository.GetPurchasesByFundAll(ctx, fundID)
}

func (u *FundUsecase) GetVirtualUsers(ctx context.Context, fundID int, offset, limit int) ([]domain.User, error) {
	return u.fundRepository.GetVirtualUsers(ctx, fundID, offset, limit)
}
func (u *FundUsecase) RemoveUser(ctx context.Context, fundID int, userID int64) error {
	return u.fundRepository.RemoveUser(ctx, fundID, userID)
}
