package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	helper "github.com/ganfay/split-core/cmd"
	"github.com/ganfay/split-core/internal/config"
	myhttp "github.com/ganfay/split-core/internal/delivery/http"
	v1 "github.com/ganfay/split-core/internal/delivery/http/v1"
	"github.com/ganfay/split-core/internal/pkg/logger"
)

// @title           My API
// @version         1.0
// @description     API documentation for my application.
//
// @host      localhost:8080
// @BasePath  /
func main() {
	ctx := context.Background()

	cfg := config.LoadConfig()

	logger.SetupLogger(cfg.Env)

	fundUC, userUC, stateUC, pool, rdb := helper.Initial(ctx, cfg)

	mux := http.NewServeMux()
	v1Handlers := v1.NewHandler(fundUC, userUC, stateUC)
	v1Handlers.RegisterRoutes(mux)
	s := myhttp.NewServer(":8080", v1.Middleware(mux))

	go func() {
		err := s.Start()
		if err != nil {
			slog.Error("Error starting server", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sign := <-quit
	slog.Info("Stopping application...", "signal", sign.String())
	err := s.Stop()
	if err != nil {
		slog.Error("Error stopping web server", "err", err)
	}
	pool.Close()
	err = rdb.Close()
	if err != nil {
		panic("Failed to close the redis database: " + err.Error())
	}
	slog.Info("Application stopped gracefully.")
}
