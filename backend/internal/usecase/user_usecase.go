package usecase

import (
	"context"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/ganfay/split-core/internal/repository"
)

type UserUsecase struct {
	repo repository.UserRepository
}

func NewUserUsecase(repo repository.UserRepository) *UserUsecase {
	return &UserUsecase{repo: repo}
}

func (u *UserUsecase) GetOrCreateRealUser(ctx context.Context, tgID *int64, username string, firstName string) (int64, error) {
	return u.repo.GetOrCreateRealUser(ctx, tgID, username, firstName)
}

func (u *UserUsecase) CreateVirtualUser(ctx context.Context, firstName string) (int64, error) {
	return u.repo.CreateVirtualUser(ctx, firstName)
}

func (u *UserUsecase) GetUserByIID(ctx context.Context, iID int64) (*domain.User, error) {
	return u.repo.GetUserByIID(ctx, iID)
}

func (u *UserUsecase) DeleteUser(ctx context.Context, iID int64) error {
	return u.repo.DeleteUser(ctx, iID)
}
