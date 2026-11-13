package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	defaultDialTimeout           = 10 * time.Second
	defaultHeartbeat             = 10 * time.Second
	defaultPrefetchCount         = 1
	defaultMaxProcessAttempts    = 3
	defaultProcessRetryBackoff   = 500 * time.Millisecond
	defaultMaxBrokerRedeliveries = int64(5)
	defaultRequeueCooldown       = 1 * time.Second
	defaultReconnectInitial      = 1 * time.Second
	defaultReconnectMax          = 30 * time.Second
)

// MsgHandler handles a single AMQP delivery payload.
type MsgHandler interface {
	HandleMessage(ctx context.Context, body []byte, Type string) error
}

type amqpChannel interface {
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	Qos(prefetchCount, prefetchSize int, global bool) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	NotifyClose(c chan *amqp.Error) chan *amqp.Error
	Close() error
}

type amqpConnection interface {
	Channel() (amqpChannel, error)
	NotifyClose(c chan *amqp.Error) chan *amqp.Error
	Close() error
}

type realAMQPConn struct {
	*amqp.Connection
}

func (r *realAMQPConn) Channel() (amqpChannel, error) {
	return r.Connection.Channel()
}

type dialFunc func(ctx context.Context, url string) (amqpConnection, error)

// Consumer consumes messages from a RabbitMQ quorum queue with retries,
// bounded dead-lettering/discarding for poison messages, automatic reconnects,
// and graceful shutdown draining.
type Consumer struct {
	url       string
	queueName string

	dialFn                dialFunc
	dialTimeout           time.Duration
	prefetchCount         int
	maxProcessAttempts    int
	processRetryBackoff   time.Duration
	maxBrokerRedeliveries int64
	requeueCooldown       time.Duration
	reconnectInitial      time.Duration
	reconnectMax          time.Duration

	mu        sync.Mutex
	conn      amqpConnection
	ch        amqpChannel
	q         amqp.Queue
	closed    bool
	closeOnce sync.Once
	closeCh   chan struct{}
}

// Option configures a Consumer.
type Option func(*Consumer)

// WithPrefetch sets the AMQP channel QoS prefetch count.
func WithPrefetch(n int) Option {
	return func(c *Consumer) {
		if n > 0 {
			c.prefetchCount = n
		}
	}
}

// WithMaxProcessAttempts sets total in-process attempts per delivery for transient errors.
func WithMaxProcessAttempts(n int) Option {
	return func(c *Consumer) {
		if n > 0 {
			c.maxProcessAttempts = n
		}
	}
}

// WithProcessRetryBackoff sets the base backoff between in-process retry attempts.
func WithProcessRetryBackoff(d time.Duration) Option {
	return func(c *Consumer) {
		if d >= 0 {
			c.processRetryBackoff = d
		}
	}
}

// WithMaxBrokerRedeliveries sets the maximum quorum queue redeliveries before discarding/dead-lettering.
func WithMaxBrokerRedeliveries(n int64) Option {
	return func(c *Consumer) {
		if n > 0 {
			c.maxBrokerRedeliveries = n
		}
	}
}

// WithRequeueCooldown sets the delay before nack(requeue=true) on transient failures.
func WithRequeueCooldown(d time.Duration) Option {
	return func(c *Consumer) {
		if d >= 0 {
			c.requeueCooldown = d
		}
	}
}

// WithReconnectBackoff sets initial and maximum delays between AMQP reconnect attempts.
func WithReconnectBackoff(initial, max time.Duration) Option {
	return func(c *Consumer) {
		if initial >= 0 {
			c.reconnectInitial = initial
		}
		if max >= initial {
			c.reconnectMax = max
		}
	}
}

// WithDialTimeout sets the TCP/AMQP handshake timeout.
func WithDialTimeout(d time.Duration) Option {
	return func(c *Consumer) {
		if d > 0 {
			c.dialTimeout = d
		}
	}
}

func withDialFunc(fn dialFunc) Option {
	return func(c *Consumer) {
		if fn != nil {
			c.dialFn = fn
		}
	}
}

func newBaseConsumer(url, queueName string, opts ...Option) (*Consumer, error) {
	if url == "" {
		return nil, errors.New("consumer: RabbitMQ URL must not be empty")
	}
	if queueName == "" {
		return nil, errors.New("consumer: queue name must not be empty")
	}

	c := &Consumer{
		url:                   url,
		queueName:             queueName,
		dialTimeout:           defaultDialTimeout,
		prefetchCount:         defaultPrefetchCount,
		maxProcessAttempts:    defaultMaxProcessAttempts,
		processRetryBackoff:   defaultProcessRetryBackoff,
		maxBrokerRedeliveries: defaultMaxBrokerRedeliveries,
		requeueCooldown:       defaultRequeueCooldown,
		reconnectInitial:      defaultReconnectInitial,
		reconnectMax:          defaultReconnectMax,
		closeCh:               make(chan struct{}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	if c.dialFn == nil {
		c.dialFn = func(ctx context.Context, targetURL string) (amqpConnection, error) {
			return defaultDialAMQP(ctx, targetURL, c.dialTimeout)
		}
	}
	return c, nil
}

// NewConsumer connects immediately to RabbitMQ, declares the quorum queue, and configures QoS.
func NewConsumer(url string, queueName string, opts ...Option) (*Consumer, error) {
	c, err := newBaseConsumer(url, queueName, opts...)
	if err != nil {
		return nil, err
	}

	conn, ch, q, err := c.openSession(context.Background())
	if err != nil {
		return nil, err
	}
	c.conn = conn
	c.ch = ch
	c.q = q
	return c, nil
}

// NewLazyConsumer creates a Consumer whose AMQP connection is established and
// retried with backoff when Start is called.
func NewLazyConsumer(url string, queueName string, opts ...Option) (*Consumer, error) {
	return newBaseConsumer(url, queueName, opts...)
}

func defaultDialAMQP(ctx context.Context, url string, timeout time.Duration) (amqpConnection, error) {
	if timeout <= 0 {
		timeout = defaultDialTimeout
	}
	cfg := amqp.Config{
		Heartbeat: defaultHeartbeat,
		Locale:    "en_US",
		Dial: func(network, addr string) (net.Conn, error) {
			dialCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			conn, err := (&net.Dialer{}).DialContext(dialCtx, network, addr)
			if err != nil {
				return nil, err
			}
			// Handshake deadline is cleared by amqp091-go once AMQP OpenOk completes.
			if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
				_ = conn.Close()
				return nil, err
			}
			return conn, nil
		},
	}

	conn, err := amqp.DialConfig(url, cfg)
	if err != nil {
		return nil, err
	}
	return &realAMQPConn{Connection: conn}, nil
}

func (c *Consumer) openSession(ctx context.Context) (amqpConnection, amqpChannel, amqp.Queue, error) {
	conn, err := c.dialFn(ctx, c.url)
	if err != nil {
		return nil, nil, amqp.Queue{}, err
	}

	ch, err := conn.Channel()
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil && !errors.Is(closeErr, amqp.ErrClosed) {
			slog.Error("failed to close RabbitMQ connection after channel error", "err", closeErr)
		}
		return nil, nil, amqp.Queue{}, err
	}

	q, err := ch.QueueDeclare(
		c.queueName, // name
		true,        // durability
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		amqp.Table{
			amqp.QueueTypeArg: amqp.QueueTypeQuorum,
		},
	)
	if err != nil {
		closeResources(ch, conn)
		return nil, nil, amqp.Queue{}, err
	}

	if err := ch.Qos(c.prefetchCount, 0, false); err != nil {
		closeResources(ch, conn)
		return nil, nil, amqp.Queue{}, err
	}

	return conn, ch, q, nil
}

func closeResources(ch amqpChannel, conn amqpConnection) error {
	var errs []error
	if ch != nil {
		if err := ch.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			slog.Warn("failed to close RabbitMQ channel", "err", err)
			errs = append(errs, err)
		}
	}
	if conn != nil {
		if err := conn.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			slog.Warn("failed to close RabbitMQ connection", "err", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Close stops consuming and cleanly closes the AMQP channel and connection.
func (c *Consumer) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})

	c.mu.Lock()
	c.closed = true
	ch := c.ch
	conn := c.conn
	c.ch = nil
	c.conn = nil
	c.mu.Unlock()

	return closeResources(ch, conn)
}

func (c *Consumer) resetSession() {
	c.mu.Lock()
	ch := c.ch
	conn := c.conn
	c.ch = nil
	c.conn = nil
	c.mu.Unlock()

	_ = closeResources(ch, conn)
}

func (c *Consumer) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *Consumer) getOrOpenSession(ctx context.Context) (amqpConnection, amqpChannel, amqp.Queue, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, nil, amqp.Queue{}, amqp.ErrClosed
	}
	if c.conn != nil && c.ch != nil {
		conn, ch, q := c.conn, c.ch, c.q
		c.mu.Unlock()
		return conn, ch, q, nil
	}
	c.mu.Unlock()

	conn, ch, q, err := c.openSession(ctx)
	if err != nil {
		return nil, nil, amqp.Queue{}, err
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = closeResources(ch, conn)
		return nil, nil, amqp.Queue{}, amqp.ErrClosed
	}
	c.conn = conn
	c.ch = ch
	c.q = q
	c.mu.Unlock()

	return conn, ch, q, nil
}

// Start runs the consumer loop, reconnecting with exponential backoff on AMQP
// disconnects and draining in-flight deliveries cleanly when ctx is canceled or Close is called.
func (c *Consumer) Start(ctx context.Context, handler MsgHandler) error {
	if c == nil {
		return errors.New("consumer: nil consumer")
	}
	if handler == nil {
		return errors.New("consumer: nil message handler")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	reconnectAttempt := 0

	for {
		if ctx.Err() != nil || c.isClosed() {
			c.resetSession()
			return nil
		}

		conn, ch, q, err := c.getOrOpenSession(ctx)
		if err != nil {
			if ctx.Err() != nil || c.isClosed() || errors.Is(err, amqp.ErrClosed) {
				c.resetSession()
				return nil
			}
			delay := c.reconnectDelay(reconnectAttempt)
			reconnectAttempt++
			slog.Error("RabbitMQ connection failed, retrying", "queue", c.queueName, "backoff", delay, "err", err)
			if !c.waitBackoff(ctx, delay) {
				c.resetSession()
				return nil
			}
			continue
		}

		reconnectAttempt = 0
		slog.Info("RabbitMQ consumer started", "queue", q.Name)

		sessionErr := c.consumeSession(ctx, ch, conn, q.Name, handler)
		c.resetSession()

		if ctx.Err() != nil || c.isClosed() {
			slog.Info("RabbitMQ consumer stopped gracefully", "queue", q.Name)
			return nil
		}

		delay := c.reconnectDelay(reconnectAttempt)
		reconnectAttempt++
		slog.Warn("RabbitMQ consumer session ended, reconnecting", "queue", q.Name, "backoff", delay, "err", sessionErr)
		if !c.waitBackoff(ctx, delay) {
			return nil
		}
	}
}

func (c *Consumer) consumeSession(ctx context.Context, ch amqpChannel, conn amqpConnection, queueName string, handler MsgHandler) error {
	msgs, err := ch.Consume(
		queueName, // queue
		"",        // consumer
		false,     // manual ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return fmt.Errorf("register consumer: %w", err)
	}

	chClose := ch.NotifyClose(make(chan *amqp.Error, 1))
	connClose := conn.NotifyClose(make(chan *amqp.Error, 1))

	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.closeCh:
			return nil
		case amqpErr, ok := <-chClose:
			if ok && amqpErr != nil {
				return fmt.Errorf("channel closed: %w", amqpErr)
			}
			return errors.New("channel closed")
		case amqpErr, ok := <-connClose:
			if ok && amqpErr != nil {
				return fmt.Errorf("connection closed: %w", amqpErr)
			}
			return errors.New("connection closed")
		case d, ok := <-msgs:
			if !ok {
				return errors.New("delivery channel closed")
			}
			if ctx.Err() != nil || c.isClosed() {
				if nackErr := d.Nack(false, true); nackErr != nil && !errors.Is(nackErr, amqp.ErrClosed) {
					slog.Warn("failed to requeue delivery during shutdown", "delivery_tag", d.DeliveryTag, "err", nackErr)
				}
				return ctx.Err()
			}
			wg.Add(1)
			go func(del amqp.Delivery) {
				defer wg.Done()
				c.handleDelivery(ctx, del, handler)
			}(d)
		}
	}
}

func (c *Consumer) handleDelivery(ctx context.Context, d amqp.Delivery, handler MsgHandler) {
	slog.Debug("Received message", "type", d.Type, "delivery_tag", d.DeliveryTag, "redelivered", d.Redelivered)

	var lastErr error
	for attempt := 0; attempt < c.maxProcessAttempts; attempt++ {
		lastErr = handler.HandleMessage(ctx, d.Body, d.Type)
		if lastErr == nil {
			if ackErr := d.Ack(false); ackErr != nil && !errors.Is(ackErr, amqp.ErrClosed) {
				slog.Warn("failed to ack message", "delivery_tag", d.DeliveryTag, "err", ackErr)
			}
			return
		}

		if ctx.Err() != nil || c.isClosed() {
			if nackErr := d.Nack(false, true); nackErr != nil && !errors.Is(nackErr, amqp.ErrClosed) {
				slog.Warn("failed to requeue interrupted message", "delivery_tag", d.DeliveryTag, "err", nackErr)
			}
			return
		}

		if isPermanentError(lastErr) {
			slog.Error("Permanent error handling message; discarding/dead-lettering",
				"type", d.Type,
				"delivery_tag", d.DeliveryTag,
				"err", lastErr,
			)
			if nackErr := d.Nack(false, false); nackErr != nil && !errors.Is(nackErr, amqp.ErrClosed) {
				slog.Warn("failed to nack permanent error message", "delivery_tag", d.DeliveryTag, "err", nackErr)
			}
			return
		}

		if attempt < c.maxProcessAttempts-1 {
			backoff := c.processDelay(attempt)
			slog.Warn("Transient error handling message; retrying in-process",
				"attempt", attempt+1,
				"max_attempts", c.maxProcessAttempts,
				"backoff", backoff,
				"err", lastErr,
			)
			if !c.waitBackoff(ctx, backoff) {
				if nackErr := d.Nack(false, true); nackErr != nil && !errors.Is(nackErr, amqp.ErrClosed) {
					slog.Warn("failed to requeue message during shutdown", "delivery_tag", d.DeliveryTag, "err", nackErr)
				}
				return
			}
		}
	}

	delCount, hasCount := deliveryCount(d)
	if (hasCount && delCount >= c.maxBrokerRedeliveries) || (!hasCount && d.Redelivered) {
		slog.Error("Max delivery attempts exhausted; discarding/dead-lettering message",
			"type", d.Type,
			"delivery_tag", d.DeliveryTag,
			"delivery_count", delCount,
			"redelivered", d.Redelivered,
			"err", lastErr,
		)
		if nackErr := d.Nack(false, false); nackErr != nil && !errors.Is(nackErr, amqp.ErrClosed) {
			slog.Warn("failed to nack exhausted message", "delivery_tag", d.DeliveryTag, "err", nackErr)
		}
		return
	}

	slog.Warn("In-process retries exhausted; requeuing message after cooldown",
		"type", d.Type,
		"delivery_tag", d.DeliveryTag,
		"delivery_count", delCount,
		"cooldown", c.requeueCooldown,
		"err", lastErr,
	)
	_ = c.waitBackoff(ctx, c.requeueCooldown)
	if nackErr := d.Nack(false, true); nackErr != nil && !errors.Is(nackErr, amqp.ErrClosed) {
		slog.Warn("failed to nack(requeue=true) message", "delivery_tag", d.DeliveryTag, "err", nackErr)
	}
}

func isPermanentError(err error) bool {
	var perm interface{ Permanent() bool }
	return errors.As(err, &perm) && perm.Permanent()
}

func deliveryCount(d amqp.Delivery) (int64, bool) {
	if d.Headers == nil {
		return 0, false
	}
	raw, ok := d.Headers["x-delivery-count"]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v > uint64(1<<63-1) {
			return int64(1<<63 - 1), true
		}
		return int64(v), true
	default:
		return 0, false
	}
}

func (c *Consumer) processDelay(attempt int) time.Duration {
	if c.processRetryBackoff <= 0 {
		return 0
	}
	d := c.processRetryBackoff
	for i := 0; i < attempt && d < 5*time.Second; i++ {
		d *= 2
	}
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

func (c *Consumer) reconnectDelay(attempt int) time.Duration {
	if c.reconnectInitial <= 0 {
		return 0
	}
	d := c.reconnectInitial
	for i := 0; i < attempt && d < c.reconnectMax; i++ {
		d *= 2
	}
	if c.reconnectMax > 0 && d > c.reconnectMax {
		d = c.reconnectMax
	}
	// Apply +/-20% jitter to avoid thundering-herd reconnect spikes.
	if d >= 5*time.Millisecond {
		jitterRange := int64(d) / 5
		if jitterRange > 0 {
			d = d - time.Duration(jitterRange) + time.Duration(rand.Int64N(2*jitterRange+1))
		}
	}
	return d
}

func (c *Consumer) waitBackoff(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil && !c.isClosed()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-c.closeCh:
		return false
	case <-timer.C:
		return true
	}
}
