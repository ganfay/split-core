package consumer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ackCall struct {
	tag      uint64
	multiple bool
	requeue  bool
	kind     string // "ack", "nack", "reject"
}

type fakeAcknowledger struct {
	mu    sync.Mutex
	calls []ackCall
	done  chan ackCall
}

func newFakeAcknowledger() *fakeAcknowledger {
	return &fakeAcknowledger{done: make(chan ackCall, 16)}
}

func (f *fakeAcknowledger) Ack(tag uint64, multiple bool) error {
	c := ackCall{tag: tag, multiple: multiple, kind: "ack"}
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
	f.done <- c
	return nil
}

func (f *fakeAcknowledger) Nack(tag uint64, multiple, requeue bool) error {
	c := ackCall{tag: tag, multiple: multiple, requeue: requeue, kind: "nack"}
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
	f.done <- c
	return nil
}

func (f *fakeAcknowledger) Reject(tag uint64, requeue bool) error {
	c := ackCall{tag: tag, requeue: requeue, kind: "reject"}
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
	f.done <- c
	return nil
}

type fakeChannel struct {
	queueDeclareErr error
	qosErr          error
	consumeErr      error
	closeErr        error

	msgs       chan amqp.Delivery
	notifyChs  []chan *amqp.Error
	mu         sync.Mutex
	autoAckArg *bool
	qosCount   int
	closed     atomic.Bool
}

func newFakeChannel() *fakeChannel {
	return &fakeChannel{
		msgs: make(chan amqp.Delivery, 16),
	}
}

func (f *fakeChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	if f.queueDeclareErr != nil {
		return amqp.Queue{}, f.queueDeclareErr
	}
	return amqp.Queue{Name: name}, nil
}

func (f *fakeChannel) Qos(prefetchCount, _ int, _ bool) error {
	f.mu.Lock()
	f.qosCount = prefetchCount
	f.mu.Unlock()
	return f.qosErr
}

func (f *fakeChannel) Consume(_ string, _ string, autoAck, _, _, _ bool, _ amqp.Table) (<-chan amqp.Delivery, error) {
	f.mu.Lock()
	f.autoAckArg = &autoAck
	f.mu.Unlock()
	if f.consumeErr != nil {
		return nil, f.consumeErr
	}
	return f.msgs, nil
}

func (f *fakeChannel) NotifyClose(c chan *amqp.Error) chan *amqp.Error {
	f.mu.Lock()
	f.notifyChs = append(f.notifyChs, c)
	f.mu.Unlock()
	return c
}

func (f *fakeChannel) Close() error {
	f.closed.Store(true)
	return f.closeErr
}

type fakeConn struct {
	ch         *fakeChannel
	channelErr error
	closeErr   error
	closed     atomic.Bool
	mu         sync.Mutex
	notifyChs  []chan *amqp.Error
}

func (f *fakeConn) Channel() (amqpChannel, error) {
	if f.channelErr != nil {
		return nil, f.channelErr
	}
	return f.ch, nil
}

func (f *fakeConn) NotifyClose(c chan *amqp.Error) chan *amqp.Error {
	f.mu.Lock()
	f.notifyChs = append(f.notifyChs, c)
	f.mu.Unlock()
	return c
}

func (f *fakeConn) Close() error {
	f.closed.Store(true)
	return f.closeErr
}

type permTestErr struct{ msg string }

func (e permTestErr) Error() string   { return e.msg }
func (e permTestErr) Permanent() bool { return true }

type handlerFunc func(ctx context.Context, body []byte, Type string) error

func (f handlerFunc) HandleMessage(ctx context.Context, body []byte, Type string) error {
	return f(ctx, body, Type)
}

func TestNewConsumerErrorHandlingAndClose(t *testing.T) {
	t.Run("validates empty url and queue", func(t *testing.T) {
		if _, err := NewConsumer("", "test"); err == nil {
			t.Fatal("expected error for empty url")
		}
		if _, err := NewConsumer("amqp://localhost", ""); err == nil {
			t.Fatal("expected error for empty queue")
		}
	})

	t.Run("QueueDeclare failure returns original error and closes channel and connection", func(t *testing.T) {
		wantErr := errors.New("queue type mismatch")
		ch := newFakeChannel()
		ch.queueDeclareErr = wantErr
		conn := &fakeConn{ch: ch}

		c, err := NewConsumer("amqp://localhost", "test", withDialFunc(func(_ context.Context, _ string) (amqpConnection, error) {
			return conn, nil
		}))
		if !errors.Is(err, wantErr) {
			t.Fatalf("NewConsumer err = %v, want %v", err, wantErr)
		}
		if c != nil {
			t.Fatal("expected nil consumer on error")
		}
		if !ch.closed.Load() || !conn.closed.Load() {
			t.Fatalf("expected channel and conn closed (ch=%v, conn=%v)", ch.closed.Load(), conn.closed.Load())
		}
	})

	t.Run("Close still closes connection when channel close fails and is idempotent", func(t *testing.T) {
		ch := newFakeChannel()
		ch.closeErr = errors.New("ch close fail")
		conn := &fakeConn{ch: ch}

		c, err := NewConsumer("amqp://localhost", "test", withDialFunc(func(_ context.Context, _ string) (amqpConnection, error) {
			return conn, nil
		}))
		if err != nil {
			t.Fatalf("NewConsumer: %v", err)
		}
		if err := c.Close(); err == nil {
			t.Fatal("expected channel close error to be returned")
		}
		if !conn.closed.Load() {
			t.Fatal("connection should be closed even when channel.Close errors")
		}
		if err := c.Close(); err != nil {
			t.Fatalf("second Close should be no-op, got %v", err)
		}
	})
}

func TestConsumerAckNackPoisonAndRetrySemantics(t *testing.T) {
	runSingleDelivery := func(t *testing.T, d amqp.Delivery, maxRedeliveries int64, handler MsgHandler) ackCall {
		t.Helper()
		ch := newFakeChannel()
		conn := &fakeConn{ch: ch}
		ack := newFakeAcknowledger()
		d.Acknowledger = ack

		c, err := NewConsumer(
			"amqp://localhost",
			"test",
			WithMaxProcessAttempts(3),
			WithProcessRetryBackoff(time.Millisecond),
			WithRequeueCooldown(time.Millisecond),
			WithMaxBrokerRedeliveries(maxRedeliveries),
			withDialFunc(func(_ context.Context, _ string) (amqpConnection, error) {
				return conn, nil
			}),
		)
		if err != nil {
			t.Fatalf("NewConsumer: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- c.Start(ctx, handler)
		}()

		ch.msgs <- d
		var gotAck ackCall
		select {
		case gotAck = <-ack.done:
		case <-time.After(2 * time.Second):
			cancel()
			t.Fatal("timed out waiting for ack/nack")
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("Start returned error: %v", err)
		}

		ch.mu.Lock()
		if ch.autoAckArg == nil || *ch.autoAckArg {
			ch.mu.Unlock()
			t.Fatal("expected Consume autoAck=false")
		}
		if ch.qosCount != 1 {
			qos := ch.qosCount
			ch.mu.Unlock()
			t.Fatalf("expected qos=1, got %d", qos)
		}
		ch.mu.Unlock()

		return gotAck
	}

	t.Run("acks on success after transient in-process retry", func(t *testing.T) {
		var attempts atomic.Int32
		ack := runSingleDelivery(t, amqp.Delivery{DeliveryTag: 1, Body: []byte(`{}`)}, 5, handlerFunc(func(_ context.Context, _ []byte, _ string) error {
			if attempts.Add(1) < 2 {
				return errors.New("transient error")
			}
			return nil
		}))
		if ack.kind != "ack" {
			t.Fatalf("expected ack, got %+v", ack)
		}
		if attempts.Load() != 2 {
			t.Fatalf("attempts = %d, want 2", attempts.Load())
		}
	})

	t.Run("immediately nacks without requeue on permanent/poison error", func(t *testing.T) {
		var attempts atomic.Int32
		ack := runSingleDelivery(t, amqp.Delivery{DeliveryTag: 2, Body: []byte(`bad`)}, 5, handlerFunc(func(_ context.Context, _ []byte, _ string) error {
			attempts.Add(1)
			return permTestErr{msg: "bad payload"}
		}))
		if ack.kind != "nack" || ack.requeue {
			t.Fatalf("expected nack(requeue=false), got %+v", ack)
		}
		if attempts.Load() != 1 {
			t.Fatalf("permanent error should not retry in-process, got %d attempts", attempts.Load())
		}
	})

	t.Run("nacks with requeue=true on transient failure when under max broker redeliveries", func(t *testing.T) {
		var attempts atomic.Int32
		ack := runSingleDelivery(t, amqp.Delivery{
			DeliveryTag: 3,
			Redelivered: true,
			Headers:     amqp.Table{"x-delivery-count": int64(2)},
		}, 5, handlerFunc(func(_ context.Context, _ []byte, _ string) error {
			attempts.Add(1)
			return errors.New("grpc unavailable")
		}))
		if ack.kind != "nack" || !ack.requeue {
			t.Fatalf("expected nack(requeue=true), got %+v", ack)
		}
		if attempts.Load() != 3 {
			t.Fatalf("attempts = %d, want 3", attempts.Load())
		}
	})

	t.Run("discards with requeue=false when x-delivery-count reaches max redeliveries", func(t *testing.T) {
		ack := runSingleDelivery(t, amqp.Delivery{
			DeliveryTag: 4,
			Redelivered: true,
			Headers:     amqp.Table{"x-delivery-count": int64(5)},
		}, 5, handlerFunc(func(_ context.Context, _ []byte, _ string) error {
			return errors.New("still unavailable")
		}))
		if ack.kind != "nack" || ack.requeue {
			t.Fatalf("expected nack(requeue=false) once delivery count exhausted, got %+v", ack)
		}
	})

	t.Run("discards redelivered message without x-delivery-count to avoid classic queue hot loop", func(t *testing.T) {
		ack := runSingleDelivery(t, amqp.Delivery{
			DeliveryTag: 5,
			Redelivered: true,
		}, 5, handlerFunc(func(_ context.Context, _ []byte, _ string) error {
			return errors.New("still failing")
		}))
		if ack.kind != "nack" || ack.requeue {
			t.Fatalf("expected nack(requeue=false) for redelivered message without delivery count, got %+v", ack)
		}
	})
}

func TestConsumerReconnectAndGracefulDrain(t *testing.T) {
	t.Run("reconnects after channel disconnect and processes subsequent message", func(t *testing.T) {
		ch1 := newFakeChannel()
		conn1 := &fakeConn{ch: ch1}
		ch2 := newFakeChannel()
		conn2 := &fakeConn{ch: ch2}

		var dialCount atomic.Int32
		c, err := NewLazyConsumer(
			"amqp://localhost",
			"test",
			WithReconnectBackoff(time.Millisecond, 5*time.Millisecond),
			withDialFunc(func(_ context.Context, _ string) (amqpConnection, error) {
				n := dialCount.Add(1)
				if n == 1 {
					return conn1, nil
				}
				return conn2, nil
			}),
		)
		if err != nil {
			t.Fatalf("NewLazyConsumer: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var handled atomic.Int32
		ack := newFakeAcknowledger()
		done := make(chan error, 1)
		go func() {
			done <- c.Start(ctx, handlerFunc(func(_ context.Context, _ []byte, _ string) error {
				handled.Add(1)
				return nil
			}))
		}()

		// Trigger disconnect on first session.
		close(ch1.msgs)

		// Deliver message on reconnected session.
		ch2.msgs <- amqp.Delivery{DeliveryTag: 10, Acknowledger: ack, Body: []byte(`ok`)}

		select {
		case got := <-ack.done:
			if got.kind != "ack" || got.tag != 10 {
				t.Fatalf("unexpected ack: %+v", got)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for message after reconnect")
		}

		cancel()
		if err := <-done; err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
		if dialCount.Load() < 2 {
			t.Fatalf("dialCount = %d, want >= 2", dialCount.Load())
		}
		if handled.Load() != 1 {
			t.Fatalf("handled = %d, want 1", handled.Load())
		}
		if !conn1.closed.Load() || !conn2.closed.Load() {
			t.Fatal("expected both old and current connections to be closed")
		}
	})

	t.Run("drains in-flight delivery before Start returns on shutdown", func(t *testing.T) {
		ch := newFakeChannel()
		conn := &fakeConn{ch: ch}
		c, err := NewConsumer("amqp://localhost", "test", withDialFunc(func(_ context.Context, _ string) (amqpConnection, error) {
			return conn, nil
		}))
		if err != nil {
			t.Fatalf("NewConsumer: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan struct{})
		release := make(chan struct{})
		var finished atomic.Bool
		ack := newFakeAcknowledger()

		done := make(chan error, 1)
		go func() {
			done <- c.Start(ctx, handlerFunc(func(_ context.Context, _ []byte, _ string) error {
				close(started)
				<-release
				finished.Store(true)
				return nil
			}))
		}()

		ch.msgs <- amqp.Delivery{DeliveryTag: 99, Acknowledger: ack}
		<-started

		cancel()
		select {
		case err := <-done:
			t.Fatalf("Start returned prematurely before in-flight handler finished (err=%v)", err)
		case <-time.After(30 * time.Millisecond):
		}

		close(release)
		if err := <-done; err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
		if !finished.Load() {
			t.Fatal("in-flight handler did not complete before shutdown")
		}
		if !ch.closed.Load() || !conn.closed.Load() {
			t.Fatal("channel and conn should be closed after graceful drain")
		}
	})

	t.Run("cancel during retry backoff aborts sleep and requeues delivery", func(t *testing.T) {
		ch := newFakeChannel()
		conn := &fakeConn{ch: ch}
		ack := newFakeAcknowledger()
		c, err := NewConsumer(
			"amqp://localhost",
			"test",
			WithMaxProcessAttempts(3),
			WithProcessRetryBackoff(5*time.Second),
			withDialFunc(func(_ context.Context, _ string) (amqpConnection, error) {
				return conn, nil
			}),
		)
		if err != nil {
			t.Fatalf("NewConsumer: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		attemptStarted := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- c.Start(ctx, handlerFunc(func(_ context.Context, _ []byte, _ string) error {
				select {
				case <-attemptStarted:
				default:
					close(attemptStarted)
				}
				return errors.New("transient failure")
			}))
		}()

		ch.msgs <- amqp.Delivery{DeliveryTag: 77, Acknowledger: ack}
		<-attemptStarted
		time.Sleep(10 * time.Millisecond)
		cancel()

		select {
		case got := <-ack.done:
			if got.kind != "nack" || !got.requeue {
				t.Fatalf("expected nack(requeue=true) on cancel during backoff, got %+v", got)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for requeue nack after cancel")
		}
		if err := <-done; err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
	})

	t.Run("NotifyClose error triggers reconnect and Close before Start skips dialing", func(t *testing.T) {
		ch1 := newFakeChannel()
		conn1 := &fakeConn{ch: ch1}
		ch2 := newFakeChannel()
		conn2 := &fakeConn{ch: ch2}
		var dials atomic.Int32
		ch1Ready := make(chan struct{})
		ch2Ready := make(chan struct{})

		c, err := NewLazyConsumer(
			"amqp://localhost",
			"test",
			WithReconnectBackoff(time.Millisecond, 5*time.Millisecond),
			withDialFunc(func(_ context.Context, _ string) (amqpConnection, error) {
				n := dials.Add(1)
				if n == 1 {
					return conn1, nil
				}
				return conn2, nil
			}),
		)
		if err != nil {
			t.Fatalf("NewLazyConsumer: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- c.Start(ctx, handlerFunc(func(_ context.Context, _ []byte, _ string) error { return nil }))
		}()

		// Wait until NotifyClose is registered on ch1, then simulate broker channel close.
		for {
			ch1.mu.Lock()
			n := len(ch1.notifyChs)
			ch1.mu.Unlock()
			if n > 0 {
				close(ch1Ready)
				break
			}
			time.Sleep(time.Millisecond)
		}
		<-ch1Ready
		ch1.mu.Lock()
		ch1.notifyChs[0] <- &amqp.Error{Code: 320, Reason: "connection forced"}
		ch1.mu.Unlock()

		// Wait until ch2 registers NotifyClose to confirm reconnect happened.
		for {
			ch2.mu.Lock()
			n := len(ch2.notifyChs)
			ch2.mu.Unlock()
			if n > 0 {
				close(ch2Ready)
				break
			}
			time.Sleep(time.Millisecond)
		}
		<-ch2Ready
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("Start returned error: %v", err)
		}

		// Close before Start should exit immediately without dialing.
		var preClosedDials atomic.Int32
		c2, _ := NewLazyConsumer("amqp://localhost", "test", withDialFunc(func(_ context.Context, _ string) (amqpConnection, error) {
			preClosedDials.Add(1)
			return &fakeConn{ch: newFakeChannel()}, nil
		}))
		_ = c2.Close()
		if err := c2.Start(context.Background(), handlerFunc(func(_ context.Context, _ []byte, _ string) error { return nil })); err != nil {
			t.Fatalf("Start on pre-closed consumer returned error: %v", err)
		}
		if preClosedDials.Load() != 0 {
			t.Fatalf("pre-closed consumer dialed %d times, want 0", preClosedDials.Load())
		}
	})
}

func TestDeliveryCountParsing(t *testing.T) {
	tests := []struct {
		name   string
		header any
		want   int64
		wantOK bool
	}{
		{name: "int64", header: int64(3), want: 3, wantOK: true},
		{name: "int32", header: int32(4), want: 4, wantOK: true},
		{name: "int", header: int(2), want: 2, wantOK: true},
		{name: "uint32", header: uint32(5), want: 5, wantOK: true},
		{name: "string invalid", header: "3", want: 0, wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := deliveryCount(amqp.Delivery{Headers: amqp.Table{"x-delivery-count": tc.header}})
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("deliveryCount() = (%d, %v), want (%d, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
