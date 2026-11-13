package telegram

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/redis/go-redis/v9"

	tele "gopkg.in/telebot.v4"
)

var kyivLocation = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		slog.Warn("Failed to load Europe/Kyiv timezone, falling back to UTC", "err", err)
		return time.UTC
	}
	return loc
}()

func (h *BotHandler) HandleCreateFund(c tele.Context) error {
	defer respondCallback(c, "create_fund")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveCtx()
	userCtx.State = domain.StateWaitFundName
	userCtx.LastMsgID = c.Message().ID
	msg := "📝 <b>Create a New Fund</b>\n\n" +
		"Send me a short name for your new fund.\n" +
		"💡 <i>Examples: Trip to Paris, BBQ Weekend, Roommates.</i>"
	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleJoinFund(c tele.Context) error {
	defer respondCallback(c, "join_fund")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveCtx()
	userCtx.State = domain.StateWaitFundJoinCode
	userCtx.LastMsgID = c.Message().ID
	msg := "🔑 <b>Join a Fund</b>\n\n" +
		"Please send me the <b>6-character invite code</b> you received from the creator."
	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleMyFund(c tele.Context) error {
	defer respondCallback(c, "my_fund")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	msg := "Your funds👇"
	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveCtx()
	userCtx.State = domain.StateFundMenu
	userCtx.LastMsgID = c.Message().ID
	menu, err := h.MyFundMenu(ctx, userCtx.InternalID, 0)
	if err != nil {
		return h.error(c, "Failed to get your funds", err.Error(), Edit)
	}
	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, msg, menu, tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleNextPreviousMF(c tele.Context) error {
	defer respondCallback(c, "next_prev_mf")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	offset, err := strconv.Atoi(c.Data())
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	if offset < 0 {
		offset = 0
	}
	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveCtx()
	userCtx.LastMsgID = c.Message().ID
	menu, err := h.MyFundMenu(ctx, userCtx.InternalID, offset)
	if err != nil {
		return h.error(c, "Failed to get your funds", err.Error(), Edit)
	}
	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, "Your funds👇", menu, tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleViewFund(c tele.Context) error {
	defer respondCallback(c, "view_fund")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	fundId, err := strconv.Atoi(c.Data())
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}
	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	userCtx.ActiveFundID = fundId
	saveCtx()
	err = h.HandleFund(c)
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}
	return nil
}

func (h *BotHandler) HandleFund(c tele.Context) error {
	defer respondCallback(c, "handle_fund")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveCtx()
	userCtx.LastMsgID = c.Message().ID
	userCtx.State = domain.StateViewFund

	fundId := userCtx.ActiveFundID
	if fundId <= 0 {
		return h.error(c, "No active fund selected", "active_fund_id <= 0", Edit)
	}
	fund := &domain.Fund{
		ID: fundId,
	}
	fund, err = h.fundUC.GetInfo(ctx, fund)
	if err != nil {
		return h.error(c, "Internal Error, failed to get info about this fund, try again later", err.Error(), Edit)
	}
	creatorDisplay := fmt.Sprintf("User_%d", fund.AuthorID)
	if author, err := h.userUC.GetUserByIID(ctx, fund.AuthorID); err == nil && author != nil {
		creatorDisplay = author.GetDisplayName()
	}

	msg := fmt.Sprintf(
		"📊 <b>Fund Dashboard:</b> %s\n"+
			"👑 <b>Creator:</b> %s\n"+
			"📅 <b>Created:</b> %s\n"+
			"🔑 <b>Invite Code:</b> <code>%s</code>\n\n"+
			"👇 <i>What would you like to do?</i>",
		html.EscapeString(fund.Name),
		html.EscapeString(creatorDisplay),
		fund.CreatedAt.In(kyivLocation).Format(`02.01.2006 15:04`),
		html.EscapeString(fund.InviteCode),
	)
	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, msg, h.FundViewMenu(userCtx.InternalID, fund.AuthorID), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleLogExpense(c tele.Context) error {
	defer respondCallback(c, "log_expense")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveCtx()
	userCtx.LastMsgID = c.Message().ID
	userCtx.State = domain.StateWaitExpense
	msg := "💸 <b>Log an Expense</b>\n\n" +
		"Send me the amount and a short description.\n\n" +
		"✍️ <b>Format:</b> <code>[PRICE] [DESCRIPTION]</code>\n" +
		"💡 <i>Example:</i> <code>150.50 Taxi to hotel</code>"
	return c.Edit(msg, h.BackMenu(), tele.ModeHTML)
}

func (h *BotHandler) HandleHistory(c tele.Context) error {
	defer respondCallback(c, "history")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	var sb strings.Builder
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveCtx()
	userCtx.LastMsgID = c.Message().ID
	userCtx.State = domain.StateViewHistory
	offset, err := strconv.Atoi(c.Data())
	if err != nil {
		return h.error(c, "Internal Error, try again later", err.Error(), Edit)
	}
	if offset < 0 {
		return h.error(c, "Internal Error, try again later", "offset < 0", Edit)
	}
	fund := &domain.Fund{
		ID: userCtx.ActiveFundID,
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
			sb.WriteString("────────────────────────────────────\n")
			fmt.Fprintf(&sb, "💰 <b>№%d • %.2f </b>\n", offset+i+1, p.Amount)
			fmt.Fprintf(&sb, "👤 Paid by: %s\n", html.EscapeString(p.Payer.GetDisplayName()))
			fmt.Fprintf(&sb, "📝 For: %s\n", html.EscapeString(p.Description))
		}
	}

	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, sb.String(), h.MenuViewFundLogs(offset, purchases), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleSettleUp(c tele.Context) error {
	defer respondCallback(c, "settle_up")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveCtx()
	userCtx.LastMsgID = c.Message().ID
	userCtx.State = domain.StateViewSettleUp

	balance, err := h.fundUC.GetBalance(ctx, userCtx.ActiveFundID)
	if err != nil {
		return h.error(c, "Internal Error, failed to calculate balance", err.Error(), Edit)
	}
	fund := &domain.Fund{
		ID: userCtx.ActiveFundID,
	}
	fund, err = h.fundUC.GetInfo(ctx, fund)
	if err != nil {
		return h.error(c, "Internal Error, failed to get fund info", err.Error(), Edit)
	}
	msg := fmt.Sprintf("⚖️ <b>Settlement for «%s»</b>\n\n", html.EscapeString(fund.Name))
	msg += fmt.Sprintf("💵 <b>Total Spent:</b> %.2f\n", balance.TotalAmount)
	msg += fmt.Sprintf("🎯 <b>Average per person:</b> %.2f\n\n\n", balance.Average)
	members, err := h.fundUC.GetMembers(ctx, userCtx.ActiveFundID)
	if err != nil {
		return h.error(c, "Internal Error, failed to load members", err.Error(), Edit)
	}
	if len(balance.Debts) == 0 {
		msg += "✅ <b>All settled up!</b> Nobody owes anything."
		slog.Debug("GetBalance", "balance", balance)
	} else {
		usernames := make(map[int64]string, len(members))
		for _, m := range members {
			usernames[m.ID] = m.GetDisplayName()
		}
		for _, debt := range balance.Debts {
			fromName := usernames[debt.FromID]
			if fromName == "" {
				fromName = fmt.Sprintf("User_%d", debt.FromID)
			}
			toName := usernames[debt.ToID]
			if toName == "" {
				toName = fmt.Sprintf("User_%d", debt.ToID)
			}
			msg += fmt.Sprintf("<b>🔴%s ➡️➡️ %.2f ➡️➡️ %s\n\n</b>", html.EscapeString(fromName), debt.Amount, html.EscapeString(toName))
		}
	}

	return c.Edit(msg, h.BackMenu(), tele.ModeHTML)
}

func (h *BotHandler) HandleMembers(c tele.Context) error {
	defer respondCallback(c, "members")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveCtx()
	userCtx.LastMsgID = c.Message().ID
	userCtx.State = domain.StateViewMembers
	members, err := h.fundUC.GetMembers(ctx, userCtx.ActiveFundID)
	if err != nil {
		return h.error(c, "Could not load members list", err.Error(), Edit)
	}

	msg := "👥 <b>Participants in this fund:</b>\n\n"
	for i, m := range members {
		name := html.EscapeString(m.FirstName) + " (" + html.EscapeString(m.GetDisplayName()) + ")"
		msg += fmt.Sprintf("%d. %s\n", i+1, name)
	}
	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, msg, h.MenuMembersView(), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleWaitAddUser(c tele.Context) error {
	defer respondCallback(c, "add_user")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, saveFunc, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveFunc()
	userCtx.LastMsgID = c.Message().ID
	userCtx.State = domain.StateWaitUsername
	msg := "<b>Enter the name of the new fund participant</b> ✍️\n\n<i>It will be added as a virtual user (not linked to Telegram).</i>"
	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleWaitRemoveUser(c tele.Context) error {
	defer respondCallback(c, "wait_remove_user")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	offset := c.Data()
	intOffset := 0
	if offset != "" {
		var err error
		intOffset, err = strconv.Atoi(offset)
		if err != nil {
			intOffset = 0
			slog.Warn("Failed to parse offset", "offset", offset)
		}
	}
	if intOffset < 0 {
		intOffset = 0
	}
	userCtx, saveFunc, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveFunc()
	userCtx.LastMsgID = c.Message().ID
	userCtx.State = domain.StateWaitToRemove
	limit := 7
	users, err := h.fundUC.GetVirtualUsers(ctx, userCtx.ActiveFundID, intOffset, limit)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	msg := "<b>Select the virtual user you want to delete</b>"
	if len(users) == 0 {
		msg += "\n\n<b>No virtual users found🫢</b>"
	}
	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, msg, h.MenuRemoveVUsers(intOffset, users), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleRemoveVUser(c tele.Context) error {
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	IIDtoDelete, err := strconv.ParseInt(c.Data(), 10, 64)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{
			Text:      "❌ Error: invalid IID user",
			ShowAlert: true,
		})
	}
	defer respondCallback(c, "remove_vuser")

	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, saveFunc, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer saveFunc()

	err = h.fundUC.RemoveUser(ctx, userCtx.ActiveFundID, IIDtoDelete)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}

	if delErr := h.userUC.DeleteUser(ctx, IIDtoDelete); delErr != nil {
		slog.Warn("Could not delete virtual user record after removing from fund", "user_id", IIDtoDelete, "err", delErr)
	}

	userCtx.State = domain.StateRemovedSuccess

	msg := "🗑 <b>Member successfully removed!</b>\n\n" +
		"<i>They are no longer part of the fund, and their expense history has been cleared.</i>"

	storedMsg := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	_, err = c.Bot().Edit(storedMsg, msg, h.BackMenu(), tele.ModeHTML)
	return err
}

func (h *BotHandler) HandleDeleteFund(c tele.Context) error {
	defer respondCallback(c, "delete_fund_confirm")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, save, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer save()
	userCtx.LastMsgID = c.Message().ID
	userCtx.State = domain.StateWaitDelete
	msg := "⚠️Are you sure want to delete this fund"
	storedMessage := &tele.Message{Chat: c.Chat(), ID: userCtx.LastMsgID}
	_, err = c.Bot().Edit(storedMessage, msg, h.SureMenu(), tele.ModeHTML)
	return err
}

func (h *BotHandler) DeleteFund(c tele.Context) error {
	defer respondCallback(c, "delete_fund")
	if c.Sender() == nil || c.Message() == nil {
		return nil
	}
	ctx, cancel := newOpCtx()
	defer cancel()

	userCtx, save, err := h.getUserCtxH(c, ctx)
	if err != nil {
		return h.error(c, "Internal error try again later", err.Error(), Edit)
	}
	defer save()
	userCtx.LastMsgID = c.Message().ID
	userCtx.State = domain.StateDelete
	err = h.fundUC.DeleteFund(ctx, userCtx.ActiveFundID, userCtx.InternalID)
	if err != nil {
		return h.error(c, "Please try again later", err.Error(), Edit)
	}
	storedMessage := &tele.Message{ID: userCtx.LastMsgID, Chat: c.Chat()}
	msg := "✅Fund has been successfully deleted"
	_, err = c.Bot().Edit(storedMessage, msg, h.BackMenu(), tele.ModeHTML)
	return err
}

func (h *BotHandler) getUserCtxH(c tele.Context, ctx context.Context) (*domain.UserContext, func(), error) {
	if c == nil || c.Sender() == nil {
		return nil, nil, errors.New("missing telegram sender")
	}
	id := &c.Sender().ID

	userCtx, err := h.statesUC.GetUserCtx(ctx, id)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			userCtx = &domain.UserContext{
				State: domain.StateNone,
			}
		} else {
			return nil, nil, fmt.Errorf("redis error: %w", err)
		}
	}

	if userCtx.InternalID == 0 {
		internalID, err := h.userUC.GetOrCreateRealUser(ctx, id, c.Sender().Username, c.Sender().FirstName)
		if err != nil {
			return nil, nil, fmt.Errorf("get or create user: %w", err)
		}
		userCtx.InternalID = internalID
	}

	saveFunc := func() {
		saveCtx, cancel := newOpCtx()
		defer cancel()
		if saveErr := h.statesUC.SaveUserCtx(saveCtx, id, userCtx); saveErr != nil {
			slog.Error("Failed to save user context", "err", saveErr)
		}
	}

	return userCtx, saveFunc, nil
}
