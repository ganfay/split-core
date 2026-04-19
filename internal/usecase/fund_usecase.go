package usecase

import (
	"SplitCore/internal/domain"
	"SplitCore/internal/pkg/utils"
	"SplitCore/internal/repository"
	"context"
	"errors"

	tele "gopkg.in/telebot.v4"
)

type FundUsecase struct {
	fr repository.FundRepository
	ur repository.UserRepository
	pr repository.PurchaseRepository
}

func NewFundUsecase(fr repository.FundRepository, ur repository.UserRepository, pr repository.PurchaseRepository) *FundUsecase {
	return &FundUsecase{fr: fr, ur: ur, pr: pr}
}

func (u *FundUsecase) GetBalance(ctx context.Context, fundID int) (*domain.Settlement, error) {
	return u.GetBalance(ctx, fundID)
}

func (u *FundUsecase) AddExpense(ctx context.Context, ctxInfoAboutPurchase tele.Context, fundID int) (*domain.Purchase, error) {
	isMember, err := u.fr.IsMember(ctx, fundID, ctxInfoAboutPurchase.Sender().ID)
	if err != nil || !isMember {
		return nil, err
	}
	cost, desc, err := utils.ParsePurchase(ctxInfoAboutPurchase.Text())
	if cost <= 0 {
		return nil, errors.New("invalid amount")
	}
	if err != nil {
		return nil, err
	}
	purchase := &domain.Purchase{
		FundID:      fundID,
		PayerID:     ctxInfoAboutPurchase.Sender().ID,
		Amount:      cost,
		Description: desc,
	}
	err = u.pr.CreatePurchase(ctx, purchase)
	if err != nil {
		return nil, err
	}
	return purchase, nil
}
