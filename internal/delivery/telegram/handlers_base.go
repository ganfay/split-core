package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/ganfay/split-core/internal/pkg/utils"

	tele "gopkg.in/telebot.v4"
)

func (h *BotHandler) HandleStart(c tele.Context) error {
	ctx := context.Background()
	var user domain.User
	user.TgID = &c.Sender().ID
	user.Username = c.Sender().Username
	user.FirstName = c.Sender().FirstName
	userStates := domain.UserContext{
		State:        domain.StateNone,
		LastMsgID:    c.Message().ID,
		ActiveFundID: -1,
	}
	err := h.statesUC.SaveUserCtx(ctx, user.TgID, &userStates)
	if err != nil {
		return err
	}

	userCtx, save, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return err
	}
	defer save()

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

		err = h.fundUC.AddMember(ctx, fund, userCtx.InternalID)
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
	ctx := context.Background()
	userCtx, save, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Failed to get user context", err.Error(), Edit)
	}
	defer save()

	slog.Debug("Handling back", "state", userCtx.State)

	switch userCtx.State {
	case domain.StateWaitExpense, domain.StateViewHistory, domain.StateViewSuccessExp, domain.StateViewSettleUp, domain.StateViewMembers:
		userCtx.State = domain.StateViewFund
		return h.HandleFund(c)

	case domain.StateViewFund:
		userCtx.State = domain.StateFundMenu
		return h.HandleMyFund(c)

	case domain.StateNone, domain.StateWaitFundName, domain.StateWaitFundJoinCode, domain.StateFundMenu:
		userCtx.State = domain.StateNone
		msg := "👋 <b>Welcome to SplitCore!</b>\n\n" +
			"I will help you and your friends easily track shared expenses and settle debts.\n\n" +
			"👇 <i>Choose an action below to get started:</i>"
		return c.Edit(msg, h.MainMenu(), tele.ModeHTML)

	default:
		panic("unhandled default case")
	}
}

func (h *BotHandler) OnText(c tele.Context) error {
	if err := c.Delete(); err != nil {
		slog.Error("error delete message", "tg_id", c.Sender().ID, "err", err.Error())
		return err
	}
	ctx := context.Background()

	userCtx, save, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Failed to get user context", err.Error(), Edit)
	}
	defer save()

	text := c.Text()
	switch userCtx.State {
	case domain.StateWaitExpense:
		storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
		cost, desc, err := utils.ParsePurchase(c.Text())
		if err != nil {
			return err
		}
		purchase, err := h.fundUC.AddExpense(ctx, userCtx.ActiveFundID, userCtx.InternalID, desc, cost)
		if err != nil {
			return h.error(c, err.Error(), err.Error(), Edit)
		}
		userCtx.State = domain.StateViewSuccessExp
		msg := fmt.Sprintf("✅You successfully added a purchase at your fund\n\nAmount💲: %.2f\nDescription📝: %s", purchase.Amount, purchase.Description)
		_, err = c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
		return err
	case domain.StateWaitFundName:
		InviteCode := utils.GenerateInviteCode(6)
		botName := os.Getenv("BOT_NAME")
		InviteCodeInviteURL := utils.GenerateInviteCodeURL(InviteCode, botName)
		fund := domain.Fund{
			AuthorID:   userCtx.InternalID,
			Name:       text,
			InviteCode: InviteCode,
		}
		_, err := h.fundUC.CreateFund(ctx, &fund)
		if err != nil {
			return h.error(c, "Failed to create fund", err.Error(), Edit)
		}
		slog.Info("Setting up fund",
			slog.Int("FundID", fund.ID),
			slog.Int64("AuthorID", userCtx.InternalID),
			slog.String("Name", fund.Name),
			slog.String("ICode", fund.InviteCode),
		)
		storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
		msg := fmt.Sprintf("Fund Created🎉!\n\nFund Code🔑: <code>%s</code>\nFund URL🌐: <code>%s</code>\n\n You can share URL or Code with users your fund👍", fund.InviteCode, InviteCodeInviteURL)
		ctxMsg, err := c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
		if err != nil {
			return h.error(c, "Failed to edit fund", err.Error(), Edit)
		}
		userCtx.LastMsgID = ctxMsg.ID
	case domain.StateWaitFundJoinCode:

		fund := &domain.Fund{
			InviteCode: text,
		}
		fund, err := h.fundUC.GetInfo(ctx, fund)
		if err != nil {
			return h.error(c, "Failed to get fund", err.Error(), Edit)
		}

		err = h.fundUC.AddMember(ctx, fund, userCtx.InternalID)
		if err != nil {
			if strings.Contains(err.Error(), "SQLSTATE 23505") {
				storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
				msg := "You already <b>exist</b> in this fund✅"
				slog.Info("User already exist in fund", "user_id", userCtx.InternalID, "fund_id", fund.ID)
				_, err = c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
				return err
			}
			return h.error(c, "Internal error, try again later", err.Error(), Edit)
		}
		storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
		msg := "You successfully joined to fund🎉\n\n" +
			"Go to <b>My Fund</b> to see this⬇️."
		ctxMsg, err := c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)

		if err != nil {
			return h.error(c, "Failed to edit fund", err.Error(), Edit)
		}
		userCtx.LastMsgID = ctxMsg.ID
		slog.Info("Setting up fund join code", "id", userCtx.InternalID)
	case domain.StateNone, domain.StateViewHistory, domain.StateFundMenu, domain.StateViewFund, domain.StateViewSettleUp, domain.StateViewMembers, domain.StateViewSuccessExp:
		storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
		msg := "No answer"
		_, err := c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
		if err != nil {
			slog.Error("error to edit message", "id", userCtx.InternalID, "err", err.Error())
			return err
		}

	default:
		panic("You have unstatement case")
	}
	userCtx.State = domain.StateNone
	return nil
}

func (h *BotHandler) error(c tele.Context, userMsg string, techMsg string, mode SendMode) error {
	slog.Error("Technical error", "msg", techMsg, "tg_id", c.Sender().ID)

	displayMsg := "⚠️ <b>Oops! Something went wrong</b>\n\n" + userMsg

	if c.Callback() != nil {
		_ = c.Respond()
	}
	ctx := context.Background()
	userCtx, err := h.statesUC.GetUserCtx(ctx, &c.Sender().ID)
	if err != nil {
		return fmt.Errorf("error getting user context: %s", err.Error())
	}
	defer func() {
		err := h.statesUC.SaveUserCtx(ctx, &c.Sender().ID, userCtx)
		if err != nil {
			slog.Error("error saving user context", "user_id", c.Sender().ID, "err", err.Error())
			return
		}
	}()

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
