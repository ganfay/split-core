package telegram

import (
	"log/slog"

	tele "gopkg.in/telebot.v4"
)

func (h *BotHandler) SetupRegister(b *tele.Bot) {
	b.Use(LoggingMiddleware())
	b.Handle("/start", h.HandleStart)
	b.Handle("\f"+CommandCreateFund, h.HandleCreateFund)
	b.Handle("\f"+CommandMyFund, h.HandleMyFund)
	b.Handle("\f"+CommandJoinFund, h.HandleJoinFund)
	b.Handle("\f"+CommandBack, h.HandleBack)
	b.Handle("\f"+CommandNext, h.HandleNextPrevious)
	b.Handle("\f"+CommandPrevious, h.HandleNextPrevious)
	b.Handle("\f"+CommandFund, h.HandleViewFund)
	b.Handle("\f"+CommandLogExpense, h.HandleLogExpense)
	b.Handle("\f"+CommandLogs, h.HandleHistory)
	b.Handle("\f"+CommandSettleUp, h.HandleSettleUp)

	b.Handle(tele.OnText, h.OnText)
	slog.Info("Setting up handlers")
}
