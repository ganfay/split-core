package telegram

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/ganfay/split-core/internal/pkg/utils"
	"github.com/redis/go-redis/v9"
	tele "gopkg.in/telebot.v4"
)

const telegramOpTimeout = 10 * time.Second

func newOpCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), telegramOpTimeout)
}

func respondCallback(c tele.Context, action string) {
	if c.Callback() == nil {
		return
	}
	if err := c.Respond(); err != nil {
		slog.Error("Failed to respond to callback", "action", action, "err", err)
	}
}

func (h *BotHandler) HandleStart(c tele.Context) error {
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userStates := domain.UserContext{
		State:        domain.StateNone,
		LastMsgID:    c.Message().ID,
		ActiveFundID: -1,
	}
	err := h.statesUC.SaveUserCtx(ctx, &c.Sender().ID, &userStates)
	if err != nil {
		return err
	}
	slog.Debug("Save userctx")
	userCtx, save, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error, try again later", err.Error(), Reply)
	}
	defer save()

	args := c.Args()
	// check for args in /start command
	if len(args) == 1 {
		arg := args[0]
		slog.Debug("Start checking args")

		splitArg := strings.SplitN(arg, "_", 2)
		if len(splitArg) != 2 || splitArg[1] == "" {
			return c.Send("⚠️ Invalid invite link format.")
		}
		switch splitArg[0] {
		case "invite":
			// if url invite code
			slog.Debug("Checking invite")
			fund := &domain.Fund{InviteCode: splitArg[1]}
			fund, err = h.fundUC.GetInfo(ctx, fund)
			if err != nil {
				return h.error(c, "Invite code not found", err.Error(), Reply)
			}
			err = h.fundUC.AddMember(ctx, fund.ID, userCtx.InternalID)
			if err != nil {
				if errors.Is(err, domain.ErrAlreadyMember) {
					return c.Reply("You already <b>exist</b> in this fund✅", h.MainMenu(), tele.ModeHTML)
				}
				return h.error(c, "Failed to join the fund", err.Error(), Reply)
			}
			msg := "Congratulations🎉\n\nYou have successfully joined to the fund😊!\n" +
				"You can see them in <b>My Funds</b>⬇️"
			return c.Reply(msg, h.MainMenu(), tele.ModeHTML)
		case "auth":
			// if url login for web version
			slog.Debug("Checking auth")

			session := domain.Session{
				Uuid:      splitArg[1],
				Status:    "authenticated",
				Firstname: c.Sender().FirstName,
				Username:  c.Sender().Username,
				IID:       userCtx.InternalID,
				TgID:      c.Sender().ID,
			}
			sessionCtx, err := h.statesUC.GetSessionCtx(ctx, splitArg[1])
			if err != nil {
				if errors.Is(err, redis.Nil) {
					return c.Reply("⚠️ Login link expired or invalid. Please request a new one on the web.", h.MainMenu(), tele.ModeHTML)
				}
				return h.error(c, "Failed to verify login session", err.Error(), Reply)
			}
			slog.Debug("Get session ctx", "status", sessionCtx.Status)
			err = h.statesUC.Session(ctx, session)
			if err != nil {
				slog.Error("[TELEGRAM] Failed to auth", "err", err)
				return h.error(c, "Failed to complete login", err.Error(), Reply)
			}

			msg := "Congratulations🎉\n\nYou have successfully login and link account!\nYou can go to web version or start using app here"
			return c.Reply(msg, h.MainMenu(), tele.ModeHTML)
		default:
			return c.Send("⚠️ Invalid invite link format.")
		}
	}
	msg := "👋 <b>Welcome to SplitCore!</b>\n\n" +
		"I will help you and your friends easily track shared expenses and settle debts.\n\n" +
		"👇 <i>Choose an action below to get started:</i>"
	return c.Reply(msg, h.MainMenu(), tele.ModeHTML)
}

func (h *BotHandler) HandleBack(c tele.Context) error {
	defer respondCallback(c, "back")
	if c.Sender() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, save, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Failed to get user context", err.Error(), Edit)
	}

	slog.Debug("Handling back", "state", userCtx.State)

	switch userCtx.State {
	case domain.StateWaitExpense, domain.StateViewHistory, domain.StateViewSuccessExp, domain.StateViewSettleUp, domain.StateViewMembers, domain.StateWaitDelete:
		userCtx.State = domain.StateViewFund
		save()
		return h.HandleFund(c)

	case domain.StateViewFund, domain.StateDelete:
		userCtx.State = domain.StateFundMenu
		save()
		return h.HandleMyFund(c)

	case domain.StateNone, domain.StateWaitFundName, domain.StateWaitFundJoinCode, domain.StateFundMenu:
		userCtx.State = domain.StateNone
		save()
		msg := "👋 <b>Welcome to SplitCore!</b>\n\n" +
			"I will help you and your friends easily track shared expenses and settle debts.\n\n" +
			"👇 <i>Choose an action below to get started:</i>"
		return c.Edit(msg, h.MainMenu(), tele.ModeHTML)

	case domain.StateWaitUsername, domain.StateSuccessAVU, domain.StateWaitToRemove:
		userCtx.State = domain.StateViewMembers
		save()
		return h.HandleMembers(c)
	case domain.StateRemovedSuccess:
		userCtx.State = domain.StateWaitToRemove
		save()
		return h.HandleWaitRemoveUser(c)
	default:
		slog.Warn("Unhandled back state, resetting to main menu", "state", userCtx.State)
		userCtx.State = domain.StateNone
		save()
		msg := "👋 <b>Welcome to SplitCore!</b>\n\n" +
			"I will help you and your friends easily track shared expenses and settle debts.\n\n" +
			"👇 <i>Choose an action below to get started:</i>"
		return c.Edit(msg, h.MainMenu(), tele.ModeHTML)
	}
}

func (h *BotHandler) OnText(c tele.Context) error {
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	if err := c.Delete(); err != nil {
		slog.Warn("error delete message", "tg_id", c.Sender().ID, "err", err.Error())
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, save, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Failed to get user context", err.Error(), Edit)
	}
	defer save()

	rawText := strings.TrimSpace(c.Text())
	if rawText == "" {
		err = errors.New("the input cannot be empty or consist solely of spaces. Please try again")
		return h.error(c, err.Error(), err.Error(), Edit)
	}

	switch userCtx.State {
	case domain.StateWaitExpense:
		if userCtx.ActiveFundID <= 0 {
			return h.error(c, "No active fund selected", "active_fund_id <= 0", Edit)
		}
		cost, desc, err := utils.ParsePurchase(rawText)
		if err != nil {
			return h.error(c, html.EscapeString(err.Error()), err.Error(), Edit)
		}
		err = h.fundUC.AddExpense(ctx, userCtx.ActiveFundID, userCtx.InternalID, desc, cost)
		if err != nil {
			return h.error(c, "Failed to add expense", err.Error(), Edit)
		}
		userCtx.State = domain.StateViewSuccessExp
		msg := fmt.Sprintf("✅You successfully added a purchase at your fund\n\nAmount💲: %.2f\nDescription📝: %s", cost, html.EscapeString(desc))
		ctxMsg, err := h.editOrSend(c, userCtx.LastMsgID, msg, h.BackMenu(), tele.ModeHTML)
		if err != nil {
			return err
		}
		if ctxMsg != nil {
			userCtx.LastMsgID = ctxMsg.ID
		}
		return nil
	case domain.StateWaitFundName:
		if utf8.RuneCountInString(rawText) > 30 {
			err = errors.New("that name is too long! Let's go with something shorter (up to 30 characters) 😅")
			return h.error(c, err.Error(), err.Error(), Edit)
		}
		inviteCode := utils.GenerateInviteCode(6)
		botName := os.Getenv("BOT_NAME")
		inviteURL := utils.GenerateInviteCodeURL(inviteCode, botName)
		fund := domain.Fund{
			AuthorID:   userCtx.InternalID,
			Name:       rawText,
			InviteCode: inviteCode,
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
		var msg string
		if botName != "" {
			msg = fmt.Sprintf("Fund Created🎉!\n\nFund Code🔑: <code>%s</code>\nFund URL🌐: <code>%s</code>\n\n You can share URL or Code with users your fund👍", html.EscapeString(fund.InviteCode), html.EscapeString(inviteURL))
		} else {
			msg = fmt.Sprintf("Fund Created🎉!\n\nFund Code🔑: <code>%s</code>\n\n You can share Code with users your fund👍", html.EscapeString(fund.InviteCode))
		}
		ctxMsg, err := h.editOrSend(c, userCtx.LastMsgID, msg, h.BackMenu(), tele.ModeHTML)
		if err != nil {
			return h.error(c, "Failed to edit fund", err.Error(), Edit)
		}
		if ctxMsg != nil {
			userCtx.LastMsgID = ctxMsg.ID
		}
	case domain.StateWaitFundJoinCode:
		if utf8.RuneCountInString(rawText) > 30 {
			err = errors.New("that invite code is too long! Please check it and try again")
			return h.error(c, err.Error(), err.Error(), Edit)
		}
		fund := &domain.Fund{
			InviteCode: rawText,
		}
		fund, err := h.fundUC.GetInfo(ctx, fund)
		if err != nil {
			return h.error(c, "Failed to get fund", err.Error(), Edit)
		}

		err = h.fundUC.AddMember(ctx, fund.ID, userCtx.InternalID)
		if err != nil {
			if errors.Is(err, domain.ErrAlreadyMember) || strings.Contains(err.Error(), "SQLSTATE 23505") {
				msg := "You already <b>exist</b> in this fund✅"
				slog.Info("User already exist in fund", "user_id", userCtx.InternalID, "fund_id", fund.ID)
				ctxMsg, editErr := h.editOrSend(c, userCtx.LastMsgID, msg, h.BackMenu(), tele.ModeHTML)
				if ctxMsg != nil {
					userCtx.LastMsgID = ctxMsg.ID
				}
				return editErr
			}
			return h.error(c, "Internal error, try again later", err.Error(), Edit)
		}
		msg := "You successfully joined to fund🎉\n\n" +
			"Go to <b>My Fund</b> to see this⬇️."
		ctxMsg, err := h.editOrSend(c, userCtx.LastMsgID, msg, h.BackMenu(), tele.ModeHTML)
		if err != nil {
			return h.error(c, "Failed to edit fund", err.Error(), Edit)
		}
		if ctxMsg != nil {
			userCtx.LastMsgID = ctxMsg.ID
		}
		slog.Info("Setting up fund join code", "id", userCtx.InternalID)
	case domain.StateWaitUsername:
		if utf8.RuneCountInString(rawText) > 30 {
			err = errors.New("that name is too long! Let's go with something shorter (up to 30 characters) 😅")
			return h.error(c, err.Error(), err.Error(), Edit)
		}
		if userCtx.ActiveFundID <= 0 {
			return h.error(c, "No active fund selected", "active_fund_id <= 0", Edit)
		}
		IID, err := h.userUC.CreateVirtualUser(ctx, rawText)
		if err != nil {
			return h.error(c, "Failed to create user", err.Error(), Edit)
		}
		err = h.fundUC.AddMember(ctx, userCtx.ActiveFundID, IID)
		if err != nil {
			slog.Debug("ADD MEMBER METHOD ERR", "err", err, "user_id", IID, "fund_id", userCtx.ActiveFundID)
			return h.error(c, "Failed to add fund", err.Error(), Edit)
		}
		userCtx.State = domain.StateSuccessAVU
		msg := fmt.Sprintf(
			"✅ Member <b>%s(IID: %d)</b> successfully added to the fund!\n\n"+
				"<i>You can now select them when logging new expenses.</i> 🧾",
			html.EscapeString(rawText), IID,
		)
		ctxMsg, err := h.editOrSend(c, userCtx.LastMsgID, msg, h.BackMenu(), tele.ModeHTML)
		if err != nil {
			return h.error(c, "Failed to edit fund", err.Error(), Edit)
		}
		if ctxMsg != nil {
			userCtx.LastMsgID = ctxMsg.ID
		}
		return nil
	case domain.StateNone, domain.StateViewHistory, domain.StateFundMenu,
		domain.StateViewFund, domain.StateViewSettleUp, domain.StateViewMembers,
		domain.StateViewSuccessExp, domain.StateWaitToRemove, domain.StateRemovedSuccess,
		domain.StateSuccessAVU, domain.StateDelete, domain.StateWaitDelete:
		msg := "No answer"
		ctxMsg, err := h.editOrSend(c, userCtx.LastMsgID, msg, h.BackMenu(), tele.ModeHTML)
		if err != nil {
			slog.Error("error to edit message", "id", userCtx.InternalID, "err", err.Error())
			return err
		}
		if ctxMsg != nil {
			userCtx.LastMsgID = ctxMsg.ID
		}

	default:
		slog.Warn("Unhandled state in OnText", "state", userCtx.State)
		userCtx.State = domain.StateNone
		return nil
	}
	userCtx.State = domain.StateNone
	return nil
}

func (h *BotHandler) editOrSend(c tele.Context, lastMsgID int, msg string, opts ...any) (*tele.Message, error) {
	if lastMsgID != 0 && c.Chat() != nil {
		storedMsg := &tele.Message{ID: lastMsgID, Chat: c.Chat()}
		return c.Bot().Edit(storedMsg, msg, opts...)
	}
	if err := c.Send(msg, opts...); err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *BotHandler) error(c tele.Context, userMsg string, techMsg string, mode SendMode) error {
	var senderID int64
	if c.Sender() != nil {
		senderID = c.Sender().ID
	}
	slog.Error("Technical error", "msg", techMsg, "tg_id", senderID)

	displayMsg := "⚠️ <b>Oops! Something went wrong</b>\n\n" + userMsg

	if c.Callback() != nil {
		_ = c.Respond()
	}

	var lastMsgID int
	if c.Sender() != nil {
		ctx, cancel := newOpCtx()
		defer cancel()
		userCtx, err := h.statesUC.GetUserCtx(ctx, &senderID)
		if err == nil && userCtx != nil {
			lastMsgID = userCtx.LastMsgID
		}
	}
	if lastMsgID == 0 && c.Callback() != nil && c.Message() != nil {
		lastMsgID = c.Message().ID
	}

	if mode == Edit && lastMsgID != 0 && c.Chat() != nil {
		storedMsg := &tele.Message{ID: lastMsgID, Chat: c.Chat()}
		_, err := c.Bot().Edit(storedMsg, displayMsg, h.BackMenu(), tele.ModeHTML)
		return err
	}
	return c.Send(displayMsg, h.BackMenu(), tele.ModeHTML)
}
