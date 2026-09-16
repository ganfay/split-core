package rabbitmq

import (
	"context"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewPublisher(url string) (*Publisher, error) {
	dial, err := amqp.Dial(url)
	if err != nil {
		slog.Error("Error to connect to RabbitMQ", "err", err)
		return nil, err
	}
	ch, err := dial.Channel()
	if err != nil {
		slog.Error("Failed to open a channel", "err", err)
		return nil, err
	}
	return &Publisher{conn: dial, ch: ch}, nil
}

func (p *Publisher) Publish(ctx context.Context, queueName string, body []byte, eventType string) error {
	q, err := p.ch.QueueDeclare(
		queueName, // name
		true,      // durability
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		amqp.Table{
			amqp.QueueTypeArg: amqp.QueueTypeQuorum,
		},
	)
	if err != nil {
		slog.Error("Failed to declare a queue", "err", err)
		return err
	}
	err = p.ch.PublishWithContext(ctx,
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Type:        eventType,
			Body:        body,
		})
	if err != nil {
		slog.Error("Failed to publish a message", "err", err)
	}
	slog.Info("[x] Sent", "body", body)

	return err
}

func (p *Publisher) Close() {
	err := p.ch.Close()
	if err != nil {
		return
	}
	err = p.conn.Close()
	if err != nil {
		return
	}
}
