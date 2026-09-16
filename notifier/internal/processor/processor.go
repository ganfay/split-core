package processor

import (
	"context"
	"encoding/json"
	"log"

	"github.com/ganfay/split-notify/internal/client"
)

type Processor struct {
	grpcClient *client.CoreClient
}

type ExpenseCreatedEvent struct {
	FundID    int64   `json:"fund_id"`
	CreatorID int64   `json:"creator_id"`
	Amount    float64 `json:"amount"`
}

func NewProcessor(grpcClient *client.CoreClient) *Processor {
	return &Processor{grpcClient: grpcClient}
}

func (p *Processor) HandleMessage(ctx context.Context, body []byte, Type string) error {
	log.Printf("Processor: Starting handle event... %s", body)
	var baseEvent ExpenseCreatedEvent

	err := json.Unmarshal(body, &baseEvent)
	if err != nil {
		return err
	}
	log.Printf("Processor: Get event! %v, Type: %s", baseEvent, Type)

	switch Type {
	case "expense_created":
		err = p.NotifyTargets(ctx, baseEvent)
		if err != nil {
			log.Printf("Processor: Error in gRPC: %v", err)
			return err
		}
	}
	return nil
}

func (p *Processor) NotifyTargets(ctx context.Context, event ExpenseCreatedEvent) error {
	resp, err := p.grpcClient.GetTargets(ctx, event.FundID, event.CreatorID)
	if err != nil {
		return err
	}
	log.Printf("Processor: response here: %v", resp)

	err = p.grpcClient.NotificateTargets(ctx, resp.GetTargets(), event.Amount, resp.GetCreatorName(), resp.GetFundName())
	if err != nil {
		return err
	}
	log.Printf("Processor: completely notified targets")
	return err
}
