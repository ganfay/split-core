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
	b.Handle("\f"+CommandNextMF, h.HandleNextPreviousMF)
	b.Handle("\f"+CommandPreviousMF, h.HandleNextPreviousMF)
	b.Handle("\f"+CommandFund, h.HandleViewFund)
	b.Handle("\f"+CommandLogExpense, h.HandleLogExpense)
	b.Handle("\f"+CommandLogs, h.HandleHistory)
	b.Handle("\f"+CommandSettleUp, h.HandleSettleUp)
	b.Handle("\f"+CommandMembers, h.HandleMembers)
	b.Handle("\f"+CommandNextVFL, h.HandleHistory)
	b.Handle("\f"+CommandPreviousVFL, h.HandleHistory)
	b.Handle(tele.OnText, h.OnText)
	slog.Info("Setting up handlers")
}
