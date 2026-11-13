package rabbitmq

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const defaultPublishTimeout = 5 * time.Second

type Publisher struct {
	mu     sync.Mutex
	url    string
	conn   *amqp.Connection
	ch     *amqp.Channel
	closed bool
}

func NewPublisher(url string) (*Publisher, error) {
	p := &Publisher{url: url}
	if err := p.connectLocked(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Publisher) connectLocked() error {
	dial, err := amqp.Dial(p.url)
	if err != nil {
		slog.Error("Error connecting to RabbitMQ", "err", err)
		return err
	}
	ch, err := dial.Channel()
	if err != nil {
		slog.Error("Failed to open a channel", "err", err)
		if closeErr := dial.Close(); closeErr != nil {
			slog.Error("Failed to close RabbitMQ connection after channel error", "err", closeErr)
		}
		return err
	}
	p.conn = dial
	p.ch = ch
	return nil
}

func (p *Publisher) ensureChannelLocked() error {
	if p.closed {
		return errors.New("publisher is closed")
	}
	if p.conn != nil && !p.conn.IsClosed() && p.ch != nil && !p.ch.IsClosed() {
		return nil
	}
	if p.ch != nil {
		_ = p.ch.Close()
		p.ch = nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
	return p.connectLocked()
}

func (p *Publisher) Publish(ctx context.Context, queueName string, body []byte, eventType string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureChannelLocked(); err != nil {
		return err
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultPublishTimeout)
		defer cancel()
	}

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
		return err
	}
	slog.Info("[x] Sent", "queue", q.Name, "type", eventType, "body", string(body))

	return nil
}

func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true
	if p.ch != nil {
		if err := p.ch.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			slog.Error("Failed to close RabbitMQ channel", "err", err)
		}
		p.ch = nil
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			slog.Error("Failed to close RabbitMQ connection", "err", err)
		}
		p.conn = nil
	}
}
