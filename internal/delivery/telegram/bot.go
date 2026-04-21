package telegram

import (
	"SplitCore/internal/domain"
	"log/slog"
	"sync"
)

type BotHandler struct {
	userCtx map[int64]*UserContext
	fundUC  domain.FundUsecase
	userUC  domain.UserUsecase
	mu      sync.RWMutex
}

func NewBotHandler(fundUC domain.FundUsecase, userUC domain.UserUsecase) *BotHandler {
	slog.Info("Setting up telegram bot")
	return &BotHandler{
		userCtx: make(map[int64]*UserContext),
		fundUC:  fundUC,
		userUC:  userUC,
	}
}

type State int

type UserContext struct {
	State        State
	LastMsgID    int
	ActiveFundID int
}

type SendMode int

const (
	Edit SendMode = iota
	Reply
	Send
)

const (
	StateNone State = iota
	StateWaitFundName
	StateWaitFundJoinCode
	StateFundMenu
	StateViewFund
	StateWaitExpense
	StateViewHistory
	StateViewSuccessExp
	StateViewSettleUp
	StateViewMembers
)
const (
	CommandCreateFund  = "create_fund"
	CommandMyFund      = "my_fund"
	CommandJoinFund    = "join_fund"
	CommandBack        = "back"
	CommandNextMF      = "next_mf"
	CommandPreviousMF  = "previous_mf"
	CommandFund        = "view_fund"
	CommandLogExpense  = "log_expense"
	CommandLogs        = "logs"
	CommandSettleUp    = "settle_up"
	CommandMembers     = "members"
	CommandPreviousVFL = "previous_vfl"
	CommandNextVFL     = "settle_up_vfl"
)
