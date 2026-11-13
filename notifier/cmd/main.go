package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ganfay/split-notify/internal/client"
	"github.com/ganfay/split-notify/internal/config"
	"github.com/ganfay/split-notify/internal/consumer"
	"github.com/ganfay/split-notify/internal/processor"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	if err := run(ctx, cfg); err != nil {
		slog.Error("notifier exited with error", "err", err)
		os.Exit(1)
	}
	slog.Info("notifier stopped gracefully")
}

func run(ctx context.Context, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("main: nil config")
	}

	coreClient, err := client.NewCoreClient(cfg.GRpcTarget())
	if err != nil {
		return fmt.Errorf("main: init core client: %w", err)
	}
	defer func() {
		if closeErr := coreClient.Close(); closeErr != nil {
			slog.Error("failed to close core client", "err", closeErr)
		}
	}()

	proc := processor.NewProcessor(coreClient)

	cons, err := consumer.NewLazyConsumer(cfg.RabbitMQ.UrlRmq(), cfg.RabbitMQ.Queue)
	if err != nil {
		return fmt.Errorf("main: init consumer: %w", err)
	}
	defer func() {
		if closeErr := cons.Close(); closeErr != nil {
			slog.Error("failed to close consumer", "err", closeErr)
		}
	}()

	return cons.Start(ctx, proc)
}
