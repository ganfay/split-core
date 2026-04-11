package main

import (
	"SplitCore/internal/delivery/telegram"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v4"
	"gopkg.in/telebot.v4/middleware"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal("Error loading .env file")
	}

	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN env var is missing")
	}

	settings := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(settings)

	if err != nil {
		log.Fatal(err)
	}
	b.Use(middleware.Recover())
	b.Use(middleware.Logger())

	h := telegram.NewBotHandler()
	h.Register(b)

	log.Println("Bot is running...")
	b.Start()
}
