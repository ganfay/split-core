package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	helper "github.com/ganfay/split-core/cmd"
	"github.com/ganfay/split-core/internal/config"
	"github.com/ganfay/split-core/internal/delivery/grpcDelivery"
	"github.com/ganfay/split-core/internal/delivery/telegram"
	"github.com/ganfay/split-core/internal/pkg/logger"
	tele "gopkg.in/telebot.v4"
)

func main() {
	ctx := context.Background()

	cfg := config.LoadConfig()

	logger.SetupLogger(cfg.Env)

	settings := tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		Client: &http.Client{Timeout: 20 * time.Second},
	}

	fundUC, userUC, stateUC, pool, rdb, publisher := helper.Initial(ctx, cfg)

	h := telegram.NewBotHandler(fundUC, userUC, stateUC)

	b, err := tele.NewBot(settings)
	if err != nil {
		slog.Error("Error creating bot", "err", err)
		pool.Close()
		publisher.Close()
		_ = rdb.Close()
		os.Exit(1)
	}
	h.SetupRegister(b)

	grpcServer := grpcDelivery.NewServer(cfg.GRpcPort, fundUC, b)

	go func() {
		if grpcErr := grpcServer.Start(); grpcErr != nil {
			slog.Error("gRPC server error", "err", grpcErr)
		}
	}()
	go func() {
		slog.Info("Starting bot", "version", cfg.BotVersion, "env", cfg.Env)
		b.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sign := <-quit
	slog.Info("Stopping application...", "signal", sign.String())
	grpcServer.Stop()
	b.Stop()
	pool.Close()
	publisher.Close()
	if closeErr := rdb.Close(); closeErr != nil {
		slog.Error("Failed to close the redis database", "err", closeErr)
	}
	slog.Info("Application stopped gracefully.")
}
