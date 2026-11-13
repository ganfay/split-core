package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/ganfay/split-notify/internal/client"
	pb "github.com/ganfay/split-proto/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultPartialRetryAttempts = 2
	defaultPartialRetryBackoff  = 200 * time.Millisecond
)

// NotificationClient defines the gRPC operations required by Processor.
type NotificationClient interface {
	GetTargets(ctx context.Context, fundID int64, creatorID int64) (*pb.TargetsResponse, error)
	NotificateTargets(ctx context.Context, targets []*pb.Target, amount float64, creatorName string, fundName string) error
}

// Processor validates and dispatches incoming notification events.
type Processor struct {
	grpcClient           NotificationClient
	partialRetryAttempts int
	partialRetryBackoff  time.Duration
}

// Option configures a Processor.
type Option func(*Processor)

// WithPartialRetry configures in-process retries for failed targets after partial delivery success.
func WithPartialRetry(attempts int, backoff time.Duration) Option {
	return func(p *Processor) {
		if attempts >= 0 {
			p.partialRetryAttempts = attempts
		}
		if backoff >= 0 {
			p.partialRetryBackoff = backoff
		}
	}
}

// ExpenseCreatedEvent represents the JSON payload of an "expense_created" event.
type ExpenseCreatedEvent struct {
	FundID    int64   `json:"fund_id"`
	CreatorID int64   `json:"creator_id"`
	Amount    float64 `json:"amount"`
}

type permanentError struct {
	err error
}

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }
func (e *permanentError) Permanent() bool {
	return true
}

// NewProcessor constructs a Processor backed by grpcClient.
func NewProcessor(grpcClient NotificationClient, opts ...Option) *Processor {
	p := &Processor{
		grpcClient:           grpcClient,
		partialRetryAttempts: defaultPartialRetryAttempts,
		partialRetryBackoff:  defaultPartialRetryBackoff,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// HandleMessage parses, validates, and processes a single queue delivery.
func (p *Processor) HandleMessage(ctx context.Context, body []byte, eventType string) error {
	if p == nil || p.grpcClient == nil {
		return errors.New("processor: nil processor or gRPC client")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	switch eventType {
	case "expense_created":
		var event ExpenseCreatedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return &permanentError{err: fmt.Errorf("processor: invalid expense_created payload: %w", err)}
		}
		if err := validateExpenseEvent(event); err != nil {
			return &permanentError{err: err}
		}
		slog.Debug("Processor: handling expense_created", "fund_id", event.FundID, "creator_id", event.CreatorID)
		return p.NotifyTargets(ctx, event)
	default:
		return &permanentError{err: fmt.Errorf("processor: unsupported event type %q", eventType)}
	}
}

// NotifyTargets resolves target members via gRPC and sends formatted notifications.
func (p *Processor) NotifyTargets(ctx context.Context, event ExpenseCreatedEvent) error {
	if p == nil || p.grpcClient == nil {
		return errors.New("processor: nil processor or gRPC client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateExpenseEvent(event); err != nil {
		return &permanentError{err: err}
	}

	resp, err := p.grpcClient.GetTargets(ctx, event.FundID, event.CreatorID)
	if err != nil {
		slog.Error("Processor: GetTargets failed", "fund_id", event.FundID, "err", err)
		return classifyGRPCError(err)
	}

	targets := resp.GetTargets()
	if len(targets) == 0 {
		slog.Debug("Processor: no targets to notify", "fund_id", event.FundID)
		return nil
	}

	err = p.grpcClient.NotificateTargets(ctx, targets, event.Amount, resp.GetCreatorName(), resp.GetFundName())
	if err == nil {
		slog.Info("Processor: notified targets", "fund_id", event.FundID, "count", len(targets))
		return nil
	}

	var delErr *client.DeliveryError
	if errors.As(err, &delErr) && delErr.Succeeded > 0 {
		return p.retryPartialFailure(ctx, delErr, event.Amount, resp.GetCreatorName(), resp.GetFundName())
	}

	return classifyGRPCError(err)
}

func (p *Processor) retryPartialFailure(ctx context.Context, delErr *client.DeliveryError, amount float64, creatorName, fundName string) error {
	totalSucceeded := delErr.Succeeded
	remaining := delErr.Failed
	lastErr := delErr.Err

	for attempt := 0; attempt < p.partialRetryAttempts && len(remaining) > 0; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if isPermanentError(lastErr) {
			break
		}
		if err := sleepWithContext(ctx, p.partialRetryBackoff); err != nil {
			return err
		}

		retryErr := p.grpcClient.NotificateTargets(ctx, remaining, amount, creatorName, fundName)
		if retryErr == nil {
			return nil
		}

		var nextDelErr *client.DeliveryError
		if errors.As(retryErr, &nextDelErr) {
			totalSucceeded += nextDelErr.Succeeded
			remaining = nextDelErr.Failed
			lastErr = nextDelErr.Err
		} else {
			lastErr = retryErr
			break
		}
	}

	// At least one recipient already received the message; mark permanent so the
	// broker does not redeliver the whole broadcast and spam prior recipients.
	return &permanentError{
		err: fmt.Errorf("processor: partial notification failure (%d succeeded, %d failed): %w", totalSucceeded, len(remaining), lastErr),
	}
}

func validateExpenseEvent(e ExpenseCreatedEvent) error {
	if e.FundID <= 0 {
		return fmt.Errorf("processor: invalid fund_id %d", e.FundID)
	}
	if e.CreatorID <= 0 {
		return fmt.Errorf("processor: invalid creator_id %d", e.CreatorID)
	}
	if math.IsNaN(e.Amount) || math.IsInf(e.Amount, 0) || e.Amount <= 0 {
		return fmt.Errorf("processor: invalid amount %v", e.Amount)
	}
	return nil
}

func classifyGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if isPermanentError(err) {
		return &permanentError{err: err}
	}
	return err
}

func isPermanentError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var perm interface{ Permanent() bool }
	if errors.As(err, &perm) && perm.Permanent() {
		return true
	}
	return allLeafErrorsPermanent(err)
}

func allLeafErrorsPermanent(err error) bool {
	if err == nil {
		return false
	}
	// Traverse wrapped and joined errors before checking gRPC status so
	// mixed permanent+transient joined errors are not misclassified.
	type multiUnwrap interface {
		Unwrap() []error
	}
	if multi, ok := err.(multiUnwrap); ok {
		errs := multi.Unwrap()
		if len(errs) == 0 {
			return false
		}
		for _, sub := range errs {
			if !allLeafErrorsPermanent(sub) {
				return false
			}
		}
		return true
	}
	if grpcErr, ok := err.(interface{ GRPCStatus() *status.Status }); ok {
		switch grpcErr.GRPCStatus().Code() {
		case codes.InvalidArgument,
			codes.NotFound,
			codes.AlreadyExists,
			codes.PermissionDenied,
			codes.FailedPrecondition,
			codes.OutOfRange,
			codes.Unimplemented,
			codes.Unauthenticated,
			codes.DataLoss:
			return true
		default:
			return false
		}
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return allLeafErrorsPermanent(unwrapped)
	}
	return false
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
