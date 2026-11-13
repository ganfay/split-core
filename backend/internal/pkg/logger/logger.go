package logger

import (
	"io"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

func SetupLogger(env string) *slog.Logger {
	var writer io.Writer = os.Stdout
	if err := os.MkdirAll("out/logs", 0755); err != nil {
		slog.Warn("Failed to create log dir, falling back to stdout", "err", err)
	} else {
		lumberjackLogger := &lumberjack.Logger{
			Filename:   "out/logs/app.log",
			MaxSize:    10,
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   true,
		}
		writer = io.MultiWriter(lumberjackLogger, os.Stdout)
	}

	var handler slog.Handler
	if env == "local" || env == "dev" || env == "development" {
		handler = slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		handler = slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
