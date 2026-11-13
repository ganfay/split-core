package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ganfay/split-notify/internal/config"
)

func TestRunGracefulCancelAndValidation(t *testing.T) {
	t.Run("nil config returns error", func(t *testing.T) {
		if err := run(context.Background(), nil); err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("canceling context stops run cleanly", func(t *testing.T) {
		// Reserve and immediately close an ephemeral port so dial fails deterministically.
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen: %v", err)
		}
		_, port, err := net.SplitHostPort(lis.Addr().String())
		if err != nil {
			t.Fatalf("SplitHostPort: %v", err)
		}
		_ = lis.Close()

		cfg := &config.Config{
			GRpcHost: "127.0.0.1",
			GRpcPort: ":50001",
			RabbitMQ: config.RabbitMQConfig{
				User:     "guest",
				Password: "guest",
				Host:     "127.0.0.1",
				Port:     port,
				Queue:    "test",
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()

		if err := run(ctx, cfg); err != nil {
			t.Fatalf("run() returned unexpected error on graceful cancel: %v", err)
		}
	})
}
