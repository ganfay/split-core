package main

import (
	"SplitCore/internal/config"
	"SplitCore/internal/delivery/telegram"
	"SplitCore/internal/repository/postgres"
	"SplitCore/internal/repository/redisRepository"
	"SplitCore/internal/usecase"
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	tele "gopkg.in/telebot.v4"
)

func main() {
	ctx := context.Background()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	cfg := config.LoadConfig()

	settings := tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

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

	fundUC := usecase.NewFundUsecase(fundRepository, purchaseRepository)
	userUC := usecase.NewUserUsecase(userRepository)
	stateUC := usecase.NewStateUsecase(StateRepository)

	h := telegram.NewBotHandler(fundUC, userUC, stateUC)

	b, err := tele.NewBot(settings)
	if err != nil {
		slog.Error("Error creating bot", "err", err)
		os.Exit(1)
	}
	h.SetupRegister(b)

	slog.Info("Starting bot", "version", cfg.BotVersion, "env", "dev")
	b.Start()
}
