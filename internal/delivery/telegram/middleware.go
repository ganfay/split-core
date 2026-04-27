package telegram

import (
	"log/slog"
	"time"

	tele "gopkg.in/telebot.v4"
)

func LoggingMiddleware() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			start := time.Now()
			user := c.Sender()
			text := c.Text()
			if c.Callback() != nil {
				text = "callback: " + c.Callback().Unique
			}
			err := next(c)

			duration := time.Since(start)

			if err != nil {
				slog.Error("request failed",
					slog.Int64("user_id", user.ID),
					slog.String("data", text),
					slog.String("duration", duration.String()),
					slog.Any("err", err),
				)
			} else {
				slog.Info("request processed",
					slog.Int64("user_id", user.ID),
					slog.String("data", text),
					slog.String("duration", duration.String()),
				)
			}

			return err
		}
	}
}
