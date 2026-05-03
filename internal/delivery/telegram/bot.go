package telegram

import (
	"log/slog"

	"github.com/ganfay/split-core/internal/domain"
)

type BotHandler struct {
	fundUC   domain.FundUsecase
	userUC   domain.UserUsecase
	statesUC domain.StatesUsecase
}

func NewBotHandler(fundUC domain.FundUsecase, userUC domain.UserUsecase, statesUC domain.StatesUsecase) *BotHandler {
	slog.Info("Setting up telegram bot")
	return &BotHandler{
		fundUC:   fundUC,
		userUC:   userUC,
		statesUC: statesUC,
	}
}

type SendMode int

const (
	Edit SendMode = iota
	Reply
	Send
)

const (
	CommandCreateFund         = "create_fund"
	CommandMyFund             = "my_fund"
	CommandJoinFund           = "join_fund"
	CommandBack               = "back"
	CommandNextMF             = "next_mf"
	CommandPreviousMF         = "previous_mf"
	CommandFund               = "view_fund"
	CommandLogExpense         = "log_expense"
	CommandLogs               = "logs"
	CommandSettleUp           = "settle_up"
	CommandMembers            = "members"
	CommandPreviousVFL        = "previous_vfl"
	CommandNextVFL            = "settle_up_vfl"
	CommandAddUser            = "add_user"
	CommandSelectToRemoveUser = "select_to_remove_user"
	CommandPrevRVU            = "prev_rvu"
	CommandNextRVU            = "next_rvu"
	CommandRemoveUser         = "remove_user"
)
