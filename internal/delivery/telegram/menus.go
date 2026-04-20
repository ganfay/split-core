package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	tele "gopkg.in/telebot.v4"
)

func (h *BotHandler) MainMenu() *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}

	btnCreateFund := menu.Data("Create Fund", CommandCreateFund)
	btnMyFund := menu.Data("My Funds", CommandMyFund)
	btnJoinFund := menu.Data("Join Fund", CommandJoinFund)

	menu.Inline(
		menu.Row(btnCreateFund),
		menu.Row(btnMyFund),
		menu.Row(btnJoinFund),
	)
	return &menu
}

func (h *BotHandler) BackMenu() *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	btnBack := menu.Data("Back", CommandBack)
	menu.Inline(menu.Row(btnBack))
	return &menu
}

func (h *BotHandler) FundViewMenu() *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	btnLogExp := menu.Data("➕ Log Expense", CommandLogExpense)
	btnLogs := menu.Data("Logs", CommandLogs)
	btnBal := menu.Data("📊 Settle Up", CommandSettleUp)
	btnMembers := menu.Data("Member", CommandMembers)
	btnBack := menu.Data("Back", CommandBack)
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
		navRow = append(navRow, menu.Data("⬅️ Prev", CommandPrevious, fmt.Sprintf("%d", prevOffset)))
	}
	if len(fundsMembers) == limit {
		nextOffset := offset + limit
		navRow = append(navRow, menu.Data("Next ➡️", CommandNext, fmt.Sprintf("%d", nextOffset)))
	}

	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}
	rows = append(rows, menu.Row(menu.Data("Back", CommandBack)))

	menu.Inline(rows...)
	h.userCtx[c.Sender().ID].LastMsgID = c.Message().ID
	return &menu
}
