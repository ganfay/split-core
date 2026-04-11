package telegram

import (
	"SplitCore/internal/domain"
	"SplitCore/internal/repository"
	"context"
	"log/slog"
	"math/rand"
	"strconv"

	tele "gopkg.in/telebot.v4"
)

type BotHandler struct {
	userState map[int64]*UserContext
	userRepo  repository.UserRepository
	fundRepo  repository.FundRepository
}

type State int

type UserContext struct {
	State     State
	LastMsgID int
}

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

func NewBotHandler(userRepository repository.UserRepository, fundRepository repository.FundRepository) *BotHandler {
	slog.Info("Setting up telegram bot")
	return &BotHandler{
		userState: make(map[int64]*UserContext),
		userRepo:  userRepository,
		fundRepo:  fundRepository,
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

func (h *BotHandler) SetupRegister(b *tele.Bot) {
	b.Use(LoggingMiddleware())
	b.Handle("/start", h.HandleStart)
	b.Handle("\f"+CommandCreateFund, h.HandleCreateFund)
	b.Handle("\f"+CommandMyFund, h.HandleMyFund)
	b.Handle("\f"+CommandJoinFund, h.HandleJoinFund)
	b.Handle("\f"+CommandBack, h.HandleBack)
	b.Handle(tele.OnText, h.OnText)
	slog.Info("Setting up handlers")
}

//-------------Handlers-----------

func (h *BotHandler) HandleStart(c tele.Context) error {
	ctx := context.Background()
	var user domain.User
	user.TgID = c.Sender().ID
	user.Username = c.Sender().Username
	user.FirstName = c.Sender().FirstName

	_, err := h.userRepo.Create(ctx, &user)
	if err != nil {
		slog.Warn(err.Error())
	}
	return c.Send("Hello, it's helper:", h.MainMenu())
}

func (h *BotHandler) HandleCreateFund(c tele.Context) error {
	id := c.Sender().ID
	if h.userState[id] == nil {
		h.userState[id] = &UserContext{}
	}

	h.userState[id].State = StateFundName
	h.userState[id].LastMsgID = c.Message().ID
	msg := c.Edit("Input Name Fund:", h.BackMenu())
	return msg
}

func (h *BotHandler) HandleBack(c tele.Context) error {
	id := c.Sender().ID
	if h.userState[id] == nil {
		h.userState[id] = &UserContext{
			State: StateNone,
		}
	}
	h.userState[id].State = StateNone
	return c.Edit("Menu:", h.MainMenu())
}

func (h *BotHandler) HandleMyFund(c tele.Context) error {
	ctx := context.Background()
	id := c.Sender().ID

	offset := "0"
	limit := "5"
	_, err := h.fundRepo.GetByUserID(ctx, id, limit, offset)
	if err != nil {
		slog.Warn(err.Error())
		return err
	}

	msg := c.Edit("Your funds:", h.BackMenu())
	return msg
}

func (h *BotHandler) HandleJoinFund(c tele.Context) error {
	id := c.Sender().ID
	if h.userState[id] == nil {
		h.userState[id] = &UserContext{
			State: StateFundJoinCode,
		}
	}
	h.userState[id].State = StateFundJoinCode
	msg := c.Edit("Input Join Code:", h.BackMenu())
	return msg
}

func (h *BotHandler) OnText(c tele.Context) error {
	err := c.Delete()
	if err != nil {
		return err
	}
	id := c.Sender().ID
	if h.userState[id] == nil {
		return nil
	}
	text := c.Text()
	ctx := context.Background()
	switch h.userState[id].State {
	case StateFundName:
		InviteCode := strconv.Itoa(rand.Int()) // 	ITS A PLUG !!!!!! REWORK LATER
		fund := domain.Fund{
			AuthorID:   id,
			Name:       text,
			InviteCode: InviteCode,
		}
		slog.Info("Setting up fund", "fund", fund)
		_, err = h.fundRepo.Create(ctx, &fund)
		if err != nil {
			slog.Warn(err.Error())
			return err
		}
		storedMsg := &tele.Message{ID: h.userState[id].LastMsgID, Chat: c.Chat()}
		_, err := c.Bot().Edit(storedMsg, "Fund Created! Fund Code: "+fund.InviteCode, h.BackMenu())
		if err != nil {
			slog.Warn(err.Error())
			return err
		}
	case StateFundJoinCode:
		slog.Info("Setting up fund join code")
	case StateNone:
		storedMsg := &tele.Message{ID: h.userState[id].LastMsgID, Chat: c.Chat()}
		_, err = c.Bot().Edit(storedMsg, "This not answer")
		if err != nil {
			slog.Warn(err.Error())
			return err
		}
	}
	return nil
}
