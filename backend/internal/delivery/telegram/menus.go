package telegram

import (
	"context"
	"fmt"
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

func (h *BotHandler) MenuRemoveVUsers(offset int, u []domain.User) *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	limit := 7
	prevOffset := offset - limit
	if prevOffset < 0 {
		prevOffset = 0
	}
	btnPrev := menu.Data("⬅️ Prev", CommandPrevRVU, strconv.Itoa(prevOffset))
	btnNext := menu.Data("Next ➡️", CommandNextRVU, strconv.Itoa(offset+limit))
	btnBack := menu.Data("⬅️⬅️Back", CommandBack)
	var rows []tele.Row
	for _, vUser := range u {
		btn := menu.Data(vUser.FirstName, CommandRemoveUser, strconv.FormatInt(vUser.ID, 10))
		rows = append(rows, menu.Row(btn))
	}
	var row []tele.Btn

	if offset > 0 {
		row = append(row, btnPrev)
	}
	if len(u) == limit {
		row = append(row, btnNext)
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, menu.Row(btnBack))
	menu.Inline(rows...)
	return &menu
}

func (h *BotHandler) MenuMembersView() *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	btnBack := menu.Data("🔙🔙Back", CommandBack)
	btnAddVirtualUser := menu.Data("➕Add Virtual User", CommandAddUser)
	btnDeleteVirtualUser := menu.Data("➖Remove Virtual User", CommandSelectToRemoveUser)
	menu.Inline(
		menu.Row(btnAddVirtualUser),
		menu.Row(btnDeleteVirtualUser),
		menu.Row(btnBack),
	)
	return &menu
}

func (h *BotHandler) MenuViewFundLogs(offset int, p []domain.Purchase) *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	limit := 7
	prevOffset := offset - limit
	if prevOffset < 0 {
		prevOffset = 0
	}
	btnPrev := menu.Data("⬅️ Prev", CommandPreviousVFL, strconv.Itoa(prevOffset))
	btnNext := menu.Data("Next ➡️", CommandNextVFL, strconv.Itoa(offset+limit))
	btnBack := menu.Data("⬅️⬅️Back", CommandBack)

	var rows []tele.Row
	var row []tele.Btn
	if offset > 0 {
		row = append(row, btnPrev)
	}
	if len(p) == limit {
		row = append(row, btnNext)
	}
	if len(row) != 0 {
		rows = append(rows, row)
	}
	rows = append(rows, menu.Row(btnBack))
	menu.Inline(rows...)
	return &menu
}

func (h *BotHandler) FundViewMenu(IID int64, fundA_IID int64) *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	btnLogExp := menu.Data("💸 Log Expense", CommandLogExpense)
	btnLogs := menu.Data("🧾 Logs", CommandLogs, "0")
	btnBal := menu.Data("⚖️ Settle Up", CommandSettleUp)
	btnMembers := menu.Data("👥 Member", CommandMembers)
	btnDelete := menu.Data("❌ Delete", CommandDelete)
	btnBack := menu.Data("🔙🔙Back", CommandBack)
	var rows []tele.Row
	rows = append(rows, menu.Row(btnLogExp))
	rows = append(rows, menu.Row(btnLogs))
	rows = append(rows, menu.Row(btnBal))
	rows = append(rows, menu.Row(btnMembers))
	if IID == fundA_IID {
		rows = append(rows, menu.Row(btnDelete))
	}
	rows = append(rows, menu.Row(btnBack))
	menu.Inline(rows...)
	return &menu
}

func (h *BotHandler) SureMenu() *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	btnYes := menu.Data("✅YES", CommandYes)
	btnNo := menu.Data("❌NO", CommandNo)

	var row []tele.Btn
	row = append(row, btnYes)
	row = append(row, btnNo)
	menu.Inline(menu.Row(row...))
	return &menu
}

func (h *BotHandler) MyFundMenu(ctx context.Context, internalID int64, offset int) (*tele.ReplyMarkup, error) {
	if offset < 0 {
		offset = 0
	}
	menu := tele.ReplyMarkup{ResizeKeyboard: true}
	limit := 5
	fundsMembers, err := h.fundUC.GetFundsByUserID(ctx, internalID, limit, offset)
	if err != nil {
		return nil, err
	}
	var rows []tele.Row
	for _, fund := range fundsMembers {
		btn := menu.Data(fund.Name, CommandFund, strconv.Itoa(fund.ID))
		rows = append(rows, menu.Row(btn))
	}
	var navRow []tele.Btn
	if offset > 0 {
		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
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
	return &menu, nil
}
