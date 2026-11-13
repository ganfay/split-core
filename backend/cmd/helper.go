package helper

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/ganfay/split-core/internal/config"
	"github.com/ganfay/split-core/internal/repository/postgres"
	"github.com/ganfay/split-core/internal/repository/rabbitmq"
	"github.com/ganfay/split-core/internal/repository/redisRepository"
	"github.com/ganfay/split-core/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const initTimeout = 10 * time.Second

func Initial(ctx context.Context, cfg *config.Config) (*usecase.FundUsecase, *usecase.UserUsecase, *usecase.StatesUsecase, *pgxpool.Pool, *redis.Client, *rabbitmq.Publisher) {
	initCtx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()

	pool, err := postgres.NewPostgresPool(initCtx, cfg.Postgres.URL())
	if err != nil {
		slog.Error("Error connecting to database", "err", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr(),
		Password:     cfg.Redis.Password,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	userRepository := postgres.NewUserRepository(pool)
	fundRepository := postgres.NewFundRepository(pool)
	purchaseRepository := postgres.NewPurchaseRepository(pool)
	stateRepository := redisRepository.NewRepository(rdb)
	publisher, err := rabbitmq.NewPublisher(cfg.RabbitMQ.UrlRmq())
	if err != nil {
		slog.Error("Error creating publisher", "err", err)
		pool.Close()
		_ = rdb.Close()
		os.Exit(1)
	}

	fundUC := usecase.NewFundUsecase(fundRepository, purchaseRepository, publisher)
	userUC := usecase.NewUserUsecase(userRepository)
	stateUC := usecase.NewStateUsecase(stateRepository)
	return fundUC, userUC, stateUC, pool, rdb, publisher
}
