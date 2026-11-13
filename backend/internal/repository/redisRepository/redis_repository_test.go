package redisRepository

import (
	"context"
	"testing"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRedisRepositoryInputGuards(t *testing.T) {
	repo := NewRepository(nil)
	ctx := context.Background()
	id := int64(1)

	require.Error(t, repo.SaveUserCtx(ctx, nil, &domain.UserContext{}))
	require.Error(t, repo.SaveUserCtx(ctx, &id, nil))
	_, err := repo.GetUserCtx(ctx, nil)
	require.Error(t, err)
	require.Error(t, repo.Session(ctx, domain.Session{}))
	_, err = repo.GetSessionCtx(ctx, "")
	require.Error(t, err)
}
