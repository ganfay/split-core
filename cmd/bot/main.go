package main

import (
	"SplitCore/internal/config"
	"SplitCore/internal/delivery/telegram"
	"SplitCore/internal/repository/postgres"
	"SplitCore/internal/usecase"
	"context"
	"log/slog"
	"os"
	"time"

	tele "gopkg.in/telebot.v4"
)

func main() {
	ctx := context.Background()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
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

	userRepository := postgres.NewUserRepository(pool)
	fundRepository := postgres.NewFundRepository(pool)
	purchaseRepository := postgres.NewPurchaseRepository(pool)

	fundUC := usecase.NewFundUsecase(fundRepository, purchaseRepository)
	userUC := usecase.NewUserUsecase(userRepository)

	h := telegram.NewBotHandler(fundUC, userUC)

	b, err := tele.NewBot(settings)
	if err != nil {
		slog.Error("Error creating bot", "err", err)
		os.Exit(1)
	}
	h.SetupRegister(b)

	slog.Info("Starting bot", "version", cfg.BotVersion, "env", "dev")
	b.Start()
}
