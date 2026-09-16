package logger

import (
	"io"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

func SetupLogger(env string) *slog.Logger {
	err := os.MkdirAll("out/logs", 0755)
	if err != nil {
		panic("Failed to create/open log dir: " + err.Error())
	}
	lumberjackLogger := &lumberjack.Logger{
		Filename:   "out/logs/app.log",
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
	}
	multiWriter := io.MultiWriter(lumberjackLogger, os.Stdout)

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
