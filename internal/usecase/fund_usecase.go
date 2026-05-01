package usecase

import (
	"context"
	"errors"
	"log/slog"
	"math"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/ganfay/split-core/internal/repository"
)

type FundUsecase struct {
	fundRepository     repository.FundRepository
	purchaseRepository repository.PurchaseRepository
}

func NewFundUsecase(fr repository.FundRepository, pr repository.PurchaseRepository) *FundUsecase {
	return &FundUsecase{fundRepository: fr, purchaseRepository: pr}
}

func (u *FundUsecase) GetBalance(ctx context.Context, fundID int) (*domain.Settlement, error) {
	slog.Debug("request", "fundID", fundID, "maxint", math.MaxInt, "ctx", ctx)
	purchases, err := u.purchaseRepository.GetPurchasesByFundAll(ctx, fundID)
	if err != nil {
		slog.Debug("GetBalance", "purchases", purchases, "error", err)
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
		m[purchase.Payer.TgID] += purchase.Amount
	}

	averageAmount := totalAmount / float64(len(members))
	var creditors []int64
	var debtors []int64
	for _, member := range members {
		m[member.TgID] = m[member.TgID] - averageAmount

		bal := m[member.TgID]
		if bal > 0.01 {
			creditors = append(creditors, member.TgID)
		} else if bal < -0.01 {
			debtors = append(debtors, member.TgID)
		}
	}
	var debts []domain.Debt
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
	var settlement = &domain.Settlement{
		TotalAmount: totalAmount,
		Debts:       debts,
		Average:     averageAmount,
	}
	return settlement
}

func (u *FundUsecase) AddExpense(ctx context.Context, fundID int, id int64, desc string, cost float64) (*domain.Purchase, error) {
	isMember, err := u.fundRepository.IsMember(ctx, fundID, id)
	if err != nil || !isMember {
		return nil, err
	}
	if cost <= 0 {
		return nil, errors.New("invalid amount")
	}
	if len(desc) > 200 {
		desc = desc[:197] + "..."
	}
	user := domain.User{
		ID: id,
	}
	purchase := &domain.Purchase{
		FundID:      fundID,
		Payer:       user,
		Amount:      cost,
		Description: desc,
	}
	err = u.purchaseRepository.CreatePurchase(ctx, purchase)
	if err != nil {
		return nil, err
	}
	return purchase, nil
}

func (u *FundUsecase) CreateFund(ctx context.Context, fund *domain.Fund) (*domain.Fund, error) {
	return u.fundRepository.CreateFund(ctx, fund)
}

func (u *FundUsecase) GetInfo(ctx context.Context, reqFund *domain.Fund) (*domain.Fund, error) {
	return u.fundRepository.GetInfo(ctx, reqFund)
}

func (u *FundUsecase) GetByUserID(ctx context.Context, userID int64, limit int, offset int) ([]domain.Fund, error) {
	return u.fundRepository.GetByUserID(ctx, userID, limit, offset)
}

func (u *FundUsecase) AddMember(ctx context.Context, fund *domain.Fund, userID int64) error {
	return u.fundRepository.AddMember(ctx, fund, userID)
}

func (u *FundUsecase) CreateMember(ctx context.Context, fund *domain.Fund, userID int64) error {
	return u.fundRepository.AddMember(ctx, fund, userID)
}

func (u *FundUsecase) IsMember(ctx context.Context, fundID int, userID int64) (bool, error) {
	return u.fundRepository.IsMember(ctx, fundID, userID)
}

func (u *FundUsecase) GetPurchasesByFundPagination(ctx context.Context, fundID int, limit int, offset int) ([]domain.Purchase, error) {
	return u.purchaseRepository.GetPurchasesByFundPagination(ctx, fundID, limit, offset)
}

func (u *FundUsecase) CreatePurchase(ctx context.Context, purchase *domain.Purchase) error {
	return u.purchaseRepository.CreatePurchase(ctx, purchase)
}

func (u *FundUsecase) GetMembers(ctx context.Context, fundID int) ([]domain.User, error) {
	return u.fundRepository.GetMembers(ctx, fundID)
}

func (u *FundUsecase) GetPurchasesByFundAll(ctx context.Context, fundID int) ([]domain.Purchase, error) {
	return u.purchaseRepository.GetPurchasesByFundAll(ctx, fundID)
}
