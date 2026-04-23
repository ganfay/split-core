package usecase

import (
	"SplitCore/internal/domain"
	"SplitCore/internal/pkg/utils"
	"SplitCore/internal/repository"
	"context"
	"errors"
	"log/slog"
	"math"
	time "time"

	tele "gopkg.in/telebot.v4"
)

type FundUsecase struct {
	fundRepository     repository.FundRepository
	purchaseRepository repository.PurchaseRepository
}

func NewFundUsecase(fr repository.FundRepository, pr repository.PurchaseRepository) *FundUsecase {
	return &FundUsecase{fundRepository: fr, purchaseRepository: pr}
}

func (u *FundUsecase) GetBalance(ctx context.Context, fundID int) (*domain.Settlement, error) {
	start := time.Now()
	slog.Debug("request", "fundID", fundID, "maxint", math.MaxInt, "start", start, "ctx", ctx)
	purchases, err := u.purchaseRepository.GetPurchasesByFundAll(ctx, fundID)
	if err != nil {
		slog.Debug("GetBalance", "purchases", purchases, "error", err)
		return nil, err
	}
	totalAmount := 0.0
	m := make(map[int64]float64)
	for _, purchase := range purchases {
		totalAmount += purchase.Amount
		m[purchase.Payer.TgID] += purchase.Amount
	}
	members, err := u.fundRepository.GetMembers(ctx, fundID)
	if err != nil {
		return nil, err
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

		m[d] += amount
		m[c] -= amount

		debt := domain.Debt{
			FromID: d,
			ToID:   c,
			Amount: amount,
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
	duration := time.Since(start)
	slog.Debug("settlements", "debts", debts, "totalAmount", totalAmount, "duration", duration, "startedAt", start, "err", err, "creditors", creditors, "purchases", purchases)
	return settlement, nil
}

func (u *FundUsecase) AddExpense(ctx context.Context, ctxInfoAboutPurchase tele.Context, fundID int) (*domain.Purchase, error) {
	isMember, err := u.fundRepository.IsMember(ctx, fundID, ctxInfoAboutPurchase.Sender().ID)
	if err != nil || !isMember {
		return nil, err
	}
	cost, desc, err := utils.ParsePurchase(ctxInfoAboutPurchase.Text())
	if err != nil {
		return nil, err
	}
	if cost <= 0 {
		return nil, errors.New("invalid amount")
	}
	if len(desc) > 200 {
		desc = desc[:197] + "..."
	}
	user := domain.User{
		TgID: ctxInfoAboutPurchase.Sender().ID,
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
