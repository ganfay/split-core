package logger

import (
	"io"
	"log/slog"
	"os"
)

func SetupLogger(env string) *slog.Logger {
	_ = os.MkdirAll("output", 0755)
	file, err := os.OpenFile("out/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("Failed to open log file: " + err.Error())
	}

	multiWriter := io.MultiWriter(file, os.Stdout)

	var handler slog.Handler
	if env == "local" || env == "dev" || env == "development" {
		handler = slog.NewTextHandler(multiWriter, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		handler = slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
