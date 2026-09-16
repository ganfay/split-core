package helper

import (
	"context"
	"log/slog"
	"os"

	"github.com/ganfay/split-core/internal/config"
	"github.com/ganfay/split-core/internal/repository/postgres"
	"github.com/ganfay/split-core/internal/repository/rabbitmq"
	"github.com/ganfay/split-core/internal/repository/redisRepository"
	"github.com/ganfay/split-core/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func Initial(ctx context.Context, cfg *config.Config) (*usecase.FundUsecase, *usecase.UserUsecase, *usecase.StatesUsecase, *pgxpool.Pool, *redis.Client) {
	pool, err := postgres.NewPostgresPool(ctx, cfg.Postgres.URL())
	if err != nil {
		slog.Error("Error connecting to database", "err", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
	})

	userRepository := postgres.NewUserRepository(pool)
	fundRepository := postgres.NewFundRepository(pool)
	purchaseRepository := postgres.NewPurchaseRepository(pool)
	StateRepository := redisRepository.NewRepository(rdb)
	publisher, err := rabbitmq.NewPublisher(cfg.RabbitMQ.UrlRmq())
	if err != nil {
		slog.Error("Error creating publisher", "err", err)
		os.Exit(1)
	}
	defer publisher.Close()

	fundUC := usecase.NewFundUsecase(fundRepository, purchaseRepository, publisher)
	userUC := usecase.NewUserUsecase(userRepository)
	stateUC := usecase.NewStateUsecase(StateRepository)
	return fundUC, userUC, stateUC, pool, rdb
}
