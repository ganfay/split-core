package telegram

import (
	"SplitCore/internal/domain"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	tele "gopkg.in/telebot.v4"
)

func (h *BotHandler) HandleCreateFund(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Failed to respond to create fund message", "err", err)
		}
	}(c)

	id := c.Sender().ID
	h.mu.Lock()
	ctxUser := h.fetchContext(id)
	ctxUser.State = StateWaitFundName
	ctxUser.LastMsgID = c.Message().ID
	h.mu.Unlock()
	msg := "📝 <b>Create a New Fund</b>\n\n" +
		"Send me a short name for your new fund.\n" +
		"💡 <i>Examples: Trip to Paris, BBQ Weekend, Roommates.</i>"
	storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
	_, err := c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleJoinFund(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Failed to respond to join fund message", "err", err)
			return
		}
	}(c)

	id := c.Sender().ID
	h.mu.Lock()
	ctxUser := h.fetchContext(id)
	ctxUser.State = StateWaitFundJoinCode
	ctxUser.LastMsgID = c.Message().ID
	h.mu.Unlock()
	msg := "🔑 <b>Join a Fund</b>\n\n" +
		"Please send me the <b>6-character invite code</b> you received from the creator."
	storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
	_, err := c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleMyFund(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Failed to respond to callback", "err", err)
			return
		}
	}(c)

	msg := "Your funds👇"
	h.mu.Lock()
	ctxUser := h.fetchContext(c.Sender().ID)
	ctxUser.State = StateFundMenu
	ctxUser.LastMsgID = c.Message().ID
	h.mu.Unlock()
	storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
	_, err := c.Bot().Edit(storedMsg, msg, h.MyFundMenu(c, 0), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleNextPreviousMF(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Failed to respond to next/previous message", "err", err)
			return
		}
	}(c)

	offset, err := strconv.Atoi(c.Data())
	h.mu.Lock()
	ctxUser := h.fetchContext(c.Sender().ID)
	ctxUser.LastMsgID = c.Message().ID
	h.mu.Unlock()
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}
	storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, h.MyFundMenu(c, offset), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleViewFund(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Failed to respond to callback", "err", err)
		}
	}(c)
	fundId, err := strconv.Atoi(c.Data())
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}
	h.mu.Lock()
	ctxUser := h.fetchContext(c.Sender().ID)
	ctxUser.ActiveFundID = fundId
	h.mu.Unlock()
	err = h.HandleFund(c)
	if err != nil {
		err = h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}
	slog.Debug("", "id", fundId)
	return err
}

func (h *BotHandler) HandleFund(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Failed to respond to view fund message", "err", err)
			return
		}
	}(c)
	h.mu.Lock()
	ctxUser := h.fetchContext(c.Sender().ID)
	ctxUser.LastMsgID = c.Message().ID
	ctxUser.State = StateViewFund
	h.mu.Unlock()
	ctx := context.Background()

	fundId := ctxUser.ActiveFundID
	fund := &domain.Fund{
		ID: fundId,
	}
	slog.Debug("", "id", fundId)

	fund, err := h.fundUC.GetInfo(ctx, fund)
	if err != nil {
		return h.error(c, "Internal Error, failed to get info about this fund, try again later", err.Error(), Edit)
	}
	author, err := h.userUC.GetUser(ctx, fund.AuthorID)
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}
	location, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}

	msg := fmt.Sprintf(
		"📊 <b>Fund Dashboard:</b> %s\n"+
			"👑 <b>Creator:</b> %s\n"+ // Позже заменишь на Имя, если сделаешь JOIN
			"📅 <b>Created:</b> %s\n"+
			"🔑 <b>Invite Code:</b> <code>%s</code>\n\n"+
			"👇 <i>What would you like to do?</i>",
		fund.Name, author.Username, fund.CreatedAt.In(location).Format(`02.01.2006 15:04`), fund.InviteCode,
	)
	storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, msg, h.FundViewMenu(), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleLogExpense(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Failed to respond to log-expense message", "err", err)
		}
	}(c)

	h.mu.Lock()
	ctxUser := h.fetchContext(c.Sender().ID)
	ctxUser.LastMsgID = c.Message().ID
	ctxUser.State = StateWaitExpense
	h.mu.Unlock()
	msg := fmt.Sprintf(
		"💸 <b>Log an Expense</b>\n\n" +
			"Send me the amount and a short description.\n\n" +
			"✍️ <b>Format:</b> <code>[PRICE] [DESCRIPTION]</code>\n" +
			"💡 <i>Example:</i> <code>150.50 Taxi to hotel</code>",
	)
	return c.Edit(msg, h.BackMenu(), tele.ModeHTML)
}

func (h *BotHandler) HandleHistory(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Failed to respond to balance message", "err", err)
		}
	}(c)
	var sb strings.Builder
	h.mu.Lock()
	ctxUser := h.fetchContext(c.Sender().ID)
	ctxUser.LastMsgID = c.Message().ID
	ctxUser.State = StateViewHistory
	h.mu.Unlock()
	offset, err := strconv.Atoi(c.Data())
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}
	if offset < 0 {
		return h.error(c, "Internal Error, try again later", fmt.Sprintf("offset < 0"), Edit)
	}
	ctx := context.Background()
	fund := &domain.Fund{
		ID: ctxUser.ActiveFundID,
	}
	fund, err = h.fundUC.GetInfo(ctx, fund)
	if err != nil {
		return h.error(c, "Internal Error, failed to get info about this fund", err.Error(), Edit)
	}
	purchases, err := h.fundUC.GetPurchasesByFundPagination(ctx, fund.ID, 7, offset)
	if err != nil {
		return h.error(c, "Internal Error, failed to get purchases", err.Error(), Edit)
	}
	sb.WriteString("<b>🧾Purchase history</b>\n\n")
	if len(purchases) == 0 {
		sb.WriteString("<i>No expenses yet.</i>")
	} else {
		for i, p := range purchases {
			sb.WriteString("───\n")
			fmt.Fprintf(&sb, "💰 <b>№%d • %.2f </b>\n", i, p.Amount)
			fmt.Fprintf(&sb, "👤 Paid by: %s\n", p.Payer.GetDisplayName())
			fmt.Fprintf(&sb, "📝 For: %s\n", p.Description)
		}
	}

	storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, sb.String(), h.MenuViewFundLogs(offset, purchases), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleSettleUp(c tele.Context) error {
	h.mu.Lock()
	ctxUser := h.fetchContext(c.Sender().ID)
	ctxUser.LastMsgID = c.Message().ID
	ctxUser.State = StateViewSettleUp
	h.mu.Unlock()
	err := c.Respond()
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}
	ctx := context.Background()
	balance, err := h.fundUC.GetBalance(ctx, ctxUser.ActiveFundID)
	if err != nil {
		return err
	}
	fund := &domain.Fund{
		ID: ctxUser.ActiveFundID,
	}
	fund, err = h.fundUC.GetInfo(ctx, fund)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("⚖️ <b>Settlement for «%s»</b>\n\n", fund.Name)
	msg += fmt.Sprintf("💵 <b>Total Spent:</b> %.2f\n", balance.TotalAmount)
	msg += fmt.Sprintf("🎯 <b>Average per person:</b> %.2f\n\n", balance.Average)
	members, err := h.fundUC.GetMembers(ctx, ctxUser.ActiveFundID)
	if err != nil {
		return err
	}
	if len(balance.Debts) == 0 {
		msg += "✅ <b>All settled up!</b> Nobody owes anything."
		slog.Debug("GetBalance", "balance", balance)
	} else {
		usernames := make(map[int64]string)
		for _, m := range members {
			usernames[m.TgID] = m.GetDisplayName()
		}
		for _, debt := range balance.Debts {
			msg += fmt.Sprintf("🔴%s ➡️➡️ %.2f ➡️➡️ %s", usernames[debt.FromID], debt.Amount, usernames[debt.ToID])
		}
	}

	return c.Edit(msg, h.BackMenu(), tele.ModeHTML)
}

func (h *BotHandler) HandleMembers(c tele.Context) error {
	h.mu.Lock()
	ctxUser := h.fetchContext(c.Sender().ID)
	ctxUser.LastMsgID = c.Message().ID
	ctxUser.State = StateViewMembers
	h.mu.Unlock()
	members, err := h.fundUC.GetMembers(context.Background(), ctxUser.ActiveFundID)
	if err != nil {
		return h.error(c, "Could not load members list", err.Error(), Edit)
	}

	msg := "👥 <b>Participants in this fund:</b>\n\n"
	for i, m := range members {
		name := m.FirstName
		name += " (" + m.GetDisplayName() + ")"
		msg += fmt.Sprintf("%d. %s\n", i+1, name)
	}

	return c.Edit(msg, h.BackMenu(), tele.ModeHTML)
}
