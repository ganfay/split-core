package usecase

import (
	"context"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/ganfay/split-core/internal/repository"
)

type StatesUsecase struct {
	redisRep repository.RedisRepository
}

func NewStateUsecase(redisRep repository.RedisRepository) *StatesUsecase {
	return &StatesUsecase{redisRep: redisRep}
}

func (r *StatesUsecase) GetUserCtx(ctx context.Context, tgID *int64) (*domain.UserContext, error) {
	return r.redisRep.GetUserCtx(ctx, tgID)
}

func (r *StatesUsecase) SaveUserCtx(ctx context.Context, tgID *int64, value *domain.UserContext) error {
	return r.redisRep.SaveUserCtx(ctx, tgID, value)
}

func (r *StatesUsecase) Session(ctx context.Context, s domain.Session) error {
	return r.redisRep.Session(ctx, s)
}

func (r *StatesUsecase) GetSessionCtx(ctx context.Context, uuid string) (domain.Session, error) {
	return r.redisRep.GetSessionCtx(ctx, uuid)
}
