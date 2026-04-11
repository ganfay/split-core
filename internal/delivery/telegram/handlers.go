package telegram

import tele "gopkg.in/telebot.v4"

type BotHandler struct {
	userState map[int64]State
}

type State int

const (
	StateNone State = iota
	StateFundName
	StateFundJoinCode
)
const (
	CommandCreateFund = "create_fund"
	CommandMyFund     = "my_fund"
	CommandJoinFund   = "join_fund"
	CommandBack       = "back"
)

func NewBotHandler() *BotHandler {
	return &BotHandler{
		userState: make(map[int64]State),
	}
}

//--------------------Menu-----------------------

func (h *BotHandler) MainMenu() *tele.ReplyMarkup {
	menu := tele.ReplyMarkup{ResizeKeyboard: true}

	btnCreateFund := menu.Data("Create Fund", CommandCreateFund)
	btnMyFund := menu.Data("My Fund", CommandMyFund)
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

//--------------Router--------------

func (h *BotHandler) Register(b *tele.Bot) {
	b.Handle("/start", h.HandleStart)
	b.Handle("\f"+CommandCreateFund, h.HandleCreateFund)
	b.Handle("\f"+CommandMyFund, h.HandleMyFund)
	b.Handle("\f"+CommandJoinFund, h.HandleJoinFund)
	b.Handle("\f"+CommandBack, h.HandleBack)
}

//-------------Handlers-----------

func (h *BotHandler) HandleStart(c tele.Context) error {
	return c.Send("Hello, it's helper:", h.MainMenu())
}

func (h *BotHandler) HandleCreateFund(c tele.Context) error {
	id := c.Callback().Sender.ID
	h.userState[id] = StateFundName
	msg := c.Edit("Input Name Fund:", h.BackMenu())
	return msg
}

func (h *BotHandler) HandleBack(c tele.Context) error {
	id := c.Callback().Sender.ID
	h.userState[id] = StateNone

	return c.Edit("Menu:", h.MainMenu())
}

func (h *BotHandler) HandleMyFund(c tele.Context) error {
	msg := c.Edit("Your funds:", h.BackMenu())
	return msg
}

func (h *BotHandler) HandleJoinFund(c tele.Context) error {
	id := c.Callback().Sender.ID
	h.userState[id] = StateFundJoinCode
	msg := c.Edit("Input Join Code:", h.BackMenu())
	return msg
}
