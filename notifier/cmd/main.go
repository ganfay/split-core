package main

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/ganfay/split-notify/internal/client"
	"github.com/ganfay/split-notify/internal/config"
	"github.com/ganfay/split-notify/internal/consumer"
	"github.com/ganfay/split-notify/internal/processor"
)

func main() {
	cfg := config.LoadConfig()

	coreClient, err := client.NewCoreClient(fmt.Sprintf("app%v", cfg.GRpcPort))
	if err != nil {
		log.Fatalln("Main: Error init coreClient")
		return
	}
	defer func(coreClient *client.CoreClient) {
		err = coreClient.Close()
		if err != nil {
			slog.Error("failed to close core client", "err", err)
		}
	}(coreClient)

	proc := processor.NewProcessor(coreClient)

	cons, err := consumer.NewConsumer(cfg.RabbitMQ.UrlRmq(), "test")
	if err != nil {
		log.Fatalf("Main: Consumer error. Error details: %v", err)
		return
	}
	defer cons.Close()
	cons.Start(proc)
}
