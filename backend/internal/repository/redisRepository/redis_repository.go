package redisRepository

import (
	"context"
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
	key := fmt.Sprintf("user:%d", *tgID)
	data, err := utils.EncodeJSON[domain.UserContext](*value)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, key, data, 24*time.Hour).Err()
}

func (r *Repository) GetUserCtx(ctx context.Context, tgID *int64) (*domain.UserContext, error) {
	key := fmt.Sprintf("user:%d", *tgID)
	data, err := r.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return &domain.UserContext{}, err
	}
	json, err := utils.DecodeJSON[domain.UserContext](data)
	return &json, err
}

func (r *Repository) Session(ctx context.Context, s domain.Session) error {
	key := fmt.Sprintf("session:%s", s.Uuid)
	json, err := utils.EncodeJSON[domain.Session](s)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, key, json, 3*time.Minute).Err()
}

func (r *Repository) GetSessionCtx(ctx context.Context, uuid string) (domain.Session, error) {

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
