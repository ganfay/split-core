package redisRepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/ganfay/split-core/internal/pkg/utils"

	"github.com/redis/go-redis/v9"
)

type Repository struct {
	rdb *redis.Client
}

func NewRepository(rdb *redis.Client) *Repository {
	return &Repository{rdb: rdb}
}

func (r *Repository) SaveUserCtx(ctx context.Context, tgID *int64, value *domain.UserContext) error {
	if tgID == nil {
		return errors.New("tgID is nil")
	}
	if value == nil {
		return errors.New("user context is nil")
	}
	key := fmt.Sprintf("user:%d", *tgID)
	data, err := utils.EncodeJSON[domain.UserContext](*value)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, key, data, 24*time.Hour).Err()
}

func (r *Repository) GetUserCtx(ctx context.Context, tgID *int64) (*domain.UserContext, error) {
	if tgID == nil {
		return &domain.UserContext{}, errors.New("tgID is nil")
	}
	key := fmt.Sprintf("user:%d", *tgID)
	data, err := r.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return &domain.UserContext{}, err
	}
	userCtx, err := utils.DecodeJSON[domain.UserContext](data)
	return &userCtx, err
}

func (r *Repository) Session(ctx context.Context, s domain.Session) error {
	if s.Uuid == "" {
		return errors.New("session uuid is empty")
	}
	key := fmt.Sprintf("session:%s", s.Uuid)
	data, err := utils.EncodeJSON[domain.Session](s)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, key, data, 3*time.Minute).Err()
}

func (r *Repository) GetSessionCtx(ctx context.Context, uuid string) (domain.Session, error) {
	if uuid == "" {
		return domain.Session{}, errors.New("session uuid is empty")
	}
	key := fmt.Sprintf("session:%s", uuid)
	data, err := r.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return domain.Session{}, err
	}
	resp, err := utils.DecodeJSON[domain.Session](data)
	if err != nil {
		return domain.Session{}, err
	}
	return resp, nil
}
