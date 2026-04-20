package main

import (
	"SplitCore/internal/delivery/telegram"
	"SplitCore/internal/repository/postgres"
	"SplitCore/internal/usecase"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v4"
)

func main() {
	ctx := context.Background()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	if err := godotenv.Load(".env"); err != nil {
		slog.Error("Error loading .env file", "error", err)
		os.Exit(1)
	}

	token := os.Getenv("TOKEN")
	if token == "" {
		slog.Error("BOT_TOKEN env var is missing")
		os.Exit(1)
	}

	settings := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	PgUser := os.Getenv("PG_USER")
	PgPass := os.Getenv("PG_PASS")
	PgDb := os.Getenv("PG_DB")
	PgHost := os.Getenv("PG_HOST")
	PgPort := os.Getenv("PG_PORT")
	if PgDb == "" || PgUser == "" || PgPass == "" || PgHost == "" || PgPort == "" {
		slog.Error("PG_USER, PG_PASSWORD and PG_DB env vars are missing")
		os.Exit(1)
	}
	url := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable", PgUser, PgPass, PgHost, PgPort, PgDb)

	pool, err := postgres.NewPostgresPool(ctx, url)
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
	vBot := os.Getenv("BOT_VER")
	slog.Info("Starting bot", "version", vBot, "env", "dev")
	b.Start()
}
