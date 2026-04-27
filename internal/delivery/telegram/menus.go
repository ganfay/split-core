package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/ganfay/split-core/internal/domain"

	tele "gopkg.in/telebot.v4"
)

func (h *BotHandler) MainMenu() *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}

	btnCreateFund := menu.Data("➕ Create New Fund", CommandCreateFund)
	btnMyFund := menu.Data("📁 My Funds", CommandMyFund)
	btnJoinFund := menu.Data("🔗 Join by Code", CommandJoinFund)

	menu.Inline(
		menu.Row(btnCreateFund),
		menu.Row(btnMyFund),
		menu.Row(btnJoinFund),
	)
	return &menu
}

func (h *BotHandler) BackMenu() *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	btnBack := menu.Data("🔙🔙Back", CommandBack)
	menu.Inline(menu.Row(btnBack))
	return &menu
}

func (h *BotHandler) MenuViewFundLogs(offset int, p []domain.Purchase) *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	limit := 7
	btnPrev := menu.Data("⬅️ Prev", CommandPreviousVFL, strconv.Itoa(offset-limit))
	btnNext := menu.Data("Next ➡️", CommandNextVFL, strconv.Itoa(offset+limit))
	btnBack := menu.Data("⬅️⬅️Back", CommandBack)

	var rows []tele.Row
	var row []tele.Btn
	if offset > 0 {
		row = append(row, btnPrev)
	}
	if len(p)-offset > limit {
		row = append(row, btnNext)
	}
	if len(row) != 0 {
		rows = append(rows, row)
	}
	rows = append(rows, menu.Row(btnBack))
	menu.Inline(rows...)
	return &menu
}

func (h *BotHandler) FundViewMenu() *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	btnLogExp := menu.Data("💸 Log Expense", CommandLogExpense)
	btnLogs := menu.Data("🧾 Logs", CommandLogs, "0")
	btnBal := menu.Data("⚖️ Settle Up", CommandSettleUp)
	btnMembers := menu.Data("👥 Member", CommandMembers)
	btnBack := menu.Data("🔙🔙Back", CommandBack)
	menu.Inline(
		menu.Row(btnLogExp),
		menu.Row(btnLogs),
		menu.Row(btnBal),
		menu.Row(btnMembers),
		menu.Row(btnBack))
	return &menu
}

func (h *BotHandler) MyFundMenu(c tele.Context, offset int) *tele.ReplyMarkup {

	ctx := context.Background()
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	limit := 5
	fundsMembers, err := h.fundUC.GetByUserID(ctx, c.Sender().ID, limit, offset)
	if err != nil {
		err := h.error(c, "Failed to get your funds", err.Error(), Edit)
		if err != nil {
			slog.Error(err.Error())
			return nil
		}
		return nil
	}
	var rows []tele.Row
	for _, fund := range fundsMembers {

		btn := menu.Data(fund.Name, CommandFund, strconv.Itoa(fund.ID))
		rows = append(rows, menu.Row(btn))
	}
	var navRow []tele.Btn
	if offset > 0 {
		prevOffset := offset - limit
		navRow = append(navRow, menu.Data("⬅️ Prev", CommandPreviousMF, fmt.Sprintf("%d", prevOffset)))
	}
	if len(fundsMembers) == limit {
		nextOffset := offset + limit
		navRow = append(navRow, menu.Data("Next ➡️", CommandNextMF, fmt.Sprintf("%d", nextOffset)))
	}

	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}
	rows = append(rows, menu.Row(menu.Data("🔙🔙Back", CommandBack)))

	menu.Inline(rows...)
	userCtx, saveCtx, err := h.getUserCtxH(c, ctx)
	if err != nil {
		_ = h.error(c, "Failed to get user context", err.Error(), Edit)
	}
	defer saveCtx()
	userCtx.LastMsgID = c.Message().ID
	return &menu
}
