package telegram

import (
	"SplitCore/internal/domain"
	"SplitCore/internal/pkg/utils"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	tele "gopkg.in/telebot.v4"
)

func (h *BotHandler) HandleStart(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Error while handling start", "err", err.Error())
		}
	}(c)
	h.getUserContext(c.Sender().ID)
	ctx := context.Background()
	var user domain.User
	user.TgID = c.Sender().ID
	user.Username = c.Sender().Username
	user.FirstName = c.Sender().FirstName

	_, err := h.userUC.CreateUser(ctx, &user)
	if err != nil {
		slog.Warn("could not register user", "err", err, "id", user.TgID)
	}
	args := c.Args()

	// if url invite code
	if len(args) == 1 {
		arg := args[0]
		if len(arg) != 6 {
			return c.Send("⚠️ Invalid invite link format.")
		}
		fund := &domain.Fund{
			InviteCode: arg,
		}
		fund, err = h.fundUC.GetInfo(ctx, fund)
		if err != nil {
			return h.error(c, "Invite code not found", err.Error(), Reply)
		}

		err = h.fundUC.AddMember(ctx, fund, user.TgID)
		if err != nil {
			return h.error(c, "Failed to join the fund", err.Error(), Reply)
		}

		msg := "Congratulations🎉\n\nYou have successfully joined to the fund😊!\n" +
			"You can see them in <b>My Funds</b>⬇️"
		return c.Reply(msg, h.MainMenu(), tele.ModeHTML)
	}
	// if url invite code
	msg := "👋 <b>Welcome to SplitCore!</b>\n\n" +
		"I will help you and your friends easily track shared expenses and settle debts.\n\n" +
		"👇 <i>Choose an action below to get started:</i>"
	return c.Reply(msg, h.MainMenu(), tele.ModeHTML)
}

func (h *BotHandler) HandleBack(c tele.Context) error {
	defer func(c tele.Context, resp ...*tele.CallbackResponse) {
		err := c.Respond(resp...)
		if err != nil {
			slog.Error("Error while handling back", "err", err.Error())
		}
	}(c)
	id := c.Sender().ID

	ctxUser := h.getUserContext(id)
	slog.Debug("Handling back", "state", ctxUser.State)

	switch ctxUser.State {
	case StateWaitExpense, StateViewHistory, StateViewSuccessExp, StateViewSettleUp, StateViewMembers:
		h.mu.Lock()
		ctxUser.State = StateViewFund
		h.mu.Unlock()
		return h.HandleFund(c)

	case StateViewFund:
		h.mu.Lock()
		ctxUser.State = StateFundMenu
		h.mu.Unlock()
		return h.HandleMyFund(c)

	case StateNone, StateWaitFundName, StateWaitFundJoinCode, StateFundMenu:
		h.mu.Lock()
		ctxUser.State = StateNone
		h.mu.Unlock()
		msg := "👋 <b>Welcome to SplitCore!</b>\n\n" +
			"I will help you and your friends easily track shared expenses and settle debts.\n\n" +
			"👇 <i>Choose an action below to get started:</i>"
		return c.Edit(msg, h.MainMenu(), tele.ModeHTML)

	default:
		panic("unhandled default case")
	}
}

func (h *BotHandler) OnText(c tele.Context) error {
	id := c.Sender().ID
	if err := c.Delete(); err != nil {
		slog.Error("error delete message", "id", id, "err", err.Error())
		return err
	}

	ctxUser := h.getUserContext(id)

	text := c.Text()
	ctx := context.Background()
	switch ctxUser.State {
	case StateWaitExpense:
		storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
		purchase, err := h.fundUC.AddExpense(ctx, c, ctxUser.ActiveFundID)
		if err != nil {
			return h.error(c, err.Error(), err.Error(), Edit)
		}
		h.mu.Lock()
		ctxUser.State = StateViewSuccessExp
		h.mu.Unlock()
		msg := fmt.Sprintf("✅You successfully added a purchase at your fund\n\nAmount💲: %.2f\nDescription📝: %s", purchase.Amount, purchase.Description)
		_, err = c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
		return err
	case StateWaitFundName:
		InviteCode := utils.GenerateInviteCode(6)
		botName := os.Getenv("BOT_NAME")
		InviteCodeInviteURL := utils.GenerateInviteCodeURL(InviteCode, botName)
		fund := domain.Fund{
			AuthorID:   id,
			Name:       text,
			InviteCode: InviteCode,
		}
		slog.Info("Setting up fund", "fund", fund, "id", id)
		_, err := h.fundUC.CreateFund(ctx, &fund)
		if err != nil {
			return h.error(c, "Failed to create fund", err.Error(), Edit)
		}
		h.mu.Lock()
		storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
		h.mu.Unlock()
		msg := fmt.Sprintf("Fund Created🎉!\n\nFund Code🔑: <code>%s</code>\nFund URL🌐: <code>%s</code>\n\n You can share URL or Code with users your fund👍", fund.InviteCode, InviteCodeInviteURL)
		ctxMsg, err := c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
		if err != nil {
			return h.error(c, "Failed to edit fund", err.Error(), Edit)
		}
		ctxUser.LastMsgID = ctxMsg.ID
	case StateWaitFundJoinCode:

		fund := &domain.Fund{
			InviteCode: text,
		}
		fund, err := h.fundUC.GetInfo(ctx, fund)
		if err != nil {
			return h.error(c, "Failed to get fund", err.Error(), Edit)
		}

		err = h.fundUC.AddMember(ctx, fund, id)
		if err != nil {
			if strings.Contains(err.Error(), "SQLSTATE 23505") {
				storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
				msg := "You already <b>exist</b> in this fund✅"
				_, err = c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
				return err
			}
			return h.error(c, "Internal error, try again later", err.Error(), Edit)
		}
		h.mu.Lock()
		storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
		h.mu.Unlock()
		msg := "You successfully joined to fund🎉\n\n" +
			"Go to <b>My Fund</b> to see this⬇️."
		ctxMsg, err := c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)

		if err != nil {
			return h.error(c, "Failed to edit fund", err.Error(), Edit)
		}
		ctxUser.LastMsgID = ctxMsg.ID
		slog.Info("Setting up fund join code", "id", id)
	case StateNone, StateViewHistory, StateFundMenu, StateViewFund, StateViewSettleUp, StateViewMembers, StateViewSuccessExp:
		storedMsg := &tele.Message{ID: ctxUser.LastMsgID, Chat: c.Chat()}
		msg := "No answer"
		_, err := c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
		if err != nil {
			slog.Error("error to edit message", "id", id, "err", err.Error())
			return err
		}

	default:
		panic("You have unstatement case")
	}
	h.mu.Lock()
	ctxUser.State = StateNone
	h.mu.Unlock()
	return nil
}

func (h *BotHandler) error(c tele.Context, userMsg string, techMsg string, mode SendMode) error {
	slog.Error("Technical error", "msg", techMsg, "user_id", c.Sender().ID)

	displayMsg := "⚠️ <b>Oops! Something went wrong</b>\n\n" + userMsg

	if c.Callback() != nil {
		_ = c.Respond()
	}

	userCtx := h.getUserContext(c.Sender().ID)
	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}

	switch mode {
	case Edit:
		if userCtx.LastMsgID != 0 {
			_, err := c.Bot().Edit(storedMsg, displayMsg, h.BackMenu(), tele.ModeHTML)
			return err
		}
		return c.Send(displayMsg, h.BackMenu(), tele.ModeHTML)
	default:
		return c.Send(displayMsg, h.BackMenu(), tele.ModeHTML)
	}
}

func (h *BotHandler) getUserContext(userID int64) *UserContext {
	h.mu.Lock()
	if h.userCtx[userID] == nil {
		h.userCtx[userID] = &UserContext{
			State: StateNone,
		}
	}
	defer h.mu.Unlock()
	return h.userCtx[userID]
}

func (h *BotHandler) fetchContext(id int64) *UserContext {
	if h.userCtx[id] == nil {
		h.userCtx[id] = &UserContext{State: StateNone}
	}
	return h.userCtx[id]
}
