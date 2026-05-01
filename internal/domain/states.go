package domain

type State int

type UserContext struct {
	State        State `json:"state"`
	LastMsgID    int   `json:"last_msg_id"`
	ActiveFundID int   `json:"active_fund_id"`
	InternalID   int64 `json:"internal_id"`
}

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
