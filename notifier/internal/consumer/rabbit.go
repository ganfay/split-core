package consumer

import (
	"context"
	"log"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type MsgHandler interface {
	HandleMessage(ctx context.Context, body []byte, Type string) error
}

type Consumer struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	q    amqp.Queue
}

func NewConsumer(url string, queueName string) (*Consumer, error) {
	dial, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := dial.Channel()
	if err != nil {
		defer func(dial *amqp.Connection) {
			err = dial.Close()
			if err != nil {
				slog.Error("failed to close RabbitMQ connection", "err", err)
			}
		}(dial)
		return nil, err
	}
	q, err := ch.QueueDeclare(
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
		err = ch.Close()
		if err != nil {
			return nil, err
		}
		err = dial.Close()
		if err != nil {
			return nil, err
		}
		return nil, err
	}
	return &Consumer{conn: dial, ch: ch, q: q}, nil
}

func (c *Consumer) Close() {
	if c.ch != nil {
		err := c.ch.Close()
		if err != nil {
			return
		}
	}
	if c.conn != nil {
		err := c.conn.Close()
		if err != nil {
			return
		}
	}
}

func (c *Consumer) Start(handler MsgHandler) {
	msgs, err := c.ch.Consume(
		c.q.Name, // queue
		"",       // consumer
		true,     // auto-ack
		false,    // exclusive
		false,    // no-local
		false,    // no-wait
		nil,      // args
	)
	if err != nil {
		log.Panicf("Failed to register a consumer: %s", err)
	}

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
			err := handler.HandleMessage(context.Background(), d.Body, d.Type)
			if err != nil {
				log.Printf("Error handling message: %v", err)
			}
		}
	}()
	log.Printf(" [*] Waiting for messages.")
	<-forever
}
