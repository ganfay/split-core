package telegram

import (
	"SplitCore/internal/domain"
	"context"
	"fmt"
	"log/slog"
	"strconv"
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
	msg := "Enter the desired fund name (any name)👇"
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
	msg := "Input Join Code🔑:\n\n" +
		"You can get an invite code by asking the fund's creator🧍‍♂️\nOr create one yourself"
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

func (h *BotHandler) HandleNextPrevious(c tele.Context) error {
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

	fund, err := h.fundRepo.GetInfo(ctx, fund)
	if err != nil {
		return h.error(c, "Internal Error, failed to get info about this fund, try again later", err.Error(), Edit)
	}
	author, err := h.userRepo.Get(ctx, fund.AuthorID)
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}
	location, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}

	msg := fmt.Sprintf("Your fund⬇️:\n\nFund name: <code>%s</code>\nAuthor: <code>%s</code>\nCreated at: <code>%s</code>\nInvite code: <code>%s</code>", fund.Name, author.Username, fund.CreatedAt.In(location).Format(`02.01.2006 15:04`), fund.InviteCode)
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
		"🖋 <b>New Expense Entry</b>\n\n" +
			"Send: <code>[price] [description]</code>\n\n" +
			"💡 <i>Tip: Use a dot for cents, e.g. 15.50</i>",
	)
	return c.Edit(msg, h.BackMenu(), tele.ModeHTML)
}

func (h *BotHandler) HandleBalance(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Failed to respond to balance message", "err", err)
		}
	}(c)
	h.mu.Lock()
	ctxUser := h.fetchContext(c.Sender().ID)
	ctxUser.LastMsgID = c.Message().ID
	ctxUser.State = StateViewBalance
	h.mu.Unlock()
	ctx := context.Background()
	fund := &domain.Fund{
		ID: ctxUser.ActiveFundID,
	}
	fund, err := h.fundRepo.GetInfo(ctx, fund)
	if err != nil {
		return h.error(c, "Internal Error, failed to get info about this fund", err.Error(), Edit)
	}
	purchases, err := h.fundRepo.GetPurchasesByFund(ctx, fund)
	if err != nil {
		return h.error(c, "Internal Error, failed to get purchases", err.Error(), Edit)
	}
	msg := "Purchases for this fund:\n\n\n"
	for _, p := range purchases {
		msg += "id:" + fmt.Sprintf("%d", p.ID) + "\n" +
			"Amount: " + fmt.Sprintf("%.2f", p.Amount) + "\n" +
			"Description: " + p.Description + "\n" +
			"PayerID: " + fmt.Sprintf("%d", p.PayerID) + "\n\n"
	}
	storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
	return err
}
