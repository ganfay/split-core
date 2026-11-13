package client

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	pb "github.com/ganfay/split-proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultRPCTimeout     = 10 * time.Second
	defaultConnectTimeout = 5 * time.Second
)

// CoreClient wraps the gRPC NotificationServiceClient with timeouts and safe formatting.
type CoreClient struct {
	client     pb.NotificationServiceClient
	conn       *grpc.ClientConn
	rpcTimeout time.Duration
}

// Option configures a CoreClient.
type Option func(*CoreClient)

// WithRPCTimeout overrides the per-RPC timeout applied when calling the gRPC server.
func WithRPCTimeout(d time.Duration) Option {
	return func(c *CoreClient) {
		if d > 0 {
			c.rpcTimeout = d
		}
	}
}

// DeliveryError reports failures encountered while sending notifications to targets.
type DeliveryError struct {
	Succeeded int
	Failed    []*pb.Target
	Err       error
}

func (e *DeliveryError) Error() string {
	return fmt.Sprintf("client: failed to notify %d target(s) (%d succeeded): %v", len(e.Failed), e.Succeeded, e.Err)
}

func (e *DeliveryError) Unwrap() error {
	return e.Err
}

// NewCoreClient creates a gRPC client for the bot core notification service.
func NewCoreClient(target string, opts ...Option) (*CoreClient, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("client: gRPC target must not be empty")
	}

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			MinConnectTimeout: defaultConnectTimeout,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("client: failed to create gRPC client: %w", err)
	}

	c := &CoreClient{
		client:     pb.NewNotificationServiceClient(conn),
		conn:       conn,
		rpcTimeout: defaultRPCTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c, nil
}

// NewCoreClientWithService constructs a CoreClient backed by an existing NotificationServiceClient.
func NewCoreClientWithService(svc pb.NotificationServiceClient, opts ...Option) (*CoreClient, error) {
	if svc == nil {
		return nil, errors.New("client: notification service client must not be nil")
	}
	c := &CoreClient{
		client:     svc,
		rpcTimeout: defaultRPCTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c, nil
}

func (c *CoreClient) withRPCTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.rpcTimeout > 0 {
		return context.WithTimeout(ctx, c.rpcTimeout)
	}
	return ctx, func() {}
}

// GetTargets fetches notification targets and metadata for a fund expense event.
func (c *CoreClient) GetTargets(ctx context.Context, fundID int64, creatorID int64) (*pb.TargetsResponse, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("client: core client is not initialized")
	}
	callCtx, cancel := c.withRPCTimeout(ctx)
	defer cancel()

	targets, err := c.client.GetNotificationTargets(callCtx, &pb.GetRequest{FundId: fundID, CreatorIid: creatorID})
	if err != nil {
		return nil, err
	}
	if targets == nil {
		return nil, errors.New("client: received nil targets response")
	}
	slog.Debug("Core: fetched notification targets", "fund_id", fundID, "count", len(targets.GetTargets()))
	return targets, nil
}

// Ping verifies connectivity to the bot core gRPC service.
func (c *CoreClient) Ping(ctx context.Context, name string) (*pb.PingReply, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("client: core client is not initialized")
	}
	callCtx, cancel := c.withRPCTimeout(ctx)
	defer cancel()

	return c.client.SayPing(callCtx, &pb.PingRequest{Name: name})
}

// FormatExpenseNotification renders a Telegram HTML notification escaping all user-supplied fields once.
func FormatExpenseNotification(firstName, creatorName string, amount float64, fundName string) string {
	safeFirstName := html.EscapeString(strings.TrimSpace(firstName))
	if safeFirstName == "" {
		safeFirstName = "there"
	}
	safeCreatorName := html.EscapeString(strings.TrimSpace(creatorName))
	if safeCreatorName == "" {
		safeCreatorName = "Unknown"
	}
	safeFundName := html.EscapeString(strings.TrimSpace(fundName))
	if safeFundName == "" {
		safeFundName = "Untitled"
	}

	return fmt.Sprintf(
		"🔔 <b>New expense in fund!</b>\n\n"+
			"Hello, <b>%s</b>! 👋\n\n"+
			"User <b>%s</b> right now added an expense in the amount of <code>%.2f</code> in fund <b>«%s»</b>. 💳",
		safeFirstName,
		safeCreatorName,
		amount,
		safeFundName,
	)
}

// NotificateTargets sends expense notifications to all valid targets and reports any failures.
func (c *CoreClient) NotificateTargets(ctx context.Context, targets []*pb.Target, amount float64, creatorName string, fundName string) error {
	if c == nil || c.client == nil {
		return errors.New("client: core client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var (
		succeeded int
		failed    []*pb.Target
		errs      []error
	)

	for i, t := range targets {
		if err := ctx.Err(); err != nil {
			for _, rem := range targets[i:] {
				if rem != nil && rem.TgId > 0 {
					failed = append(failed, rem)
				}
			}
			errs = append(errs, err)
			break
		}

		if t == nil || t.TgId <= 0 {
			slog.Warn("skipping notification target without valid telegram id")
			continue
		}

		text := FormatExpenseNotification(t.FirstName, creatorName, amount, fundName)

		callCtx, cancel := c.withRPCTimeout(ctx)
		reply, err := c.client.NotificateTarget(callCtx, &pb.NotTarget{Msg: text, TgId: t.TgId})
		cancel()

		if err == nil && (reply == nil || !reply.GetSuccess()) {
			err = errors.New("notification rejected by server (success=false)")
		}
		if err != nil {
			slog.Error("failed to send notification", "tg_id", t.TgId, "err", err)
			failed = append(failed, t)
			errs = append(errs, fmt.Errorf("tg_id %d: %w", t.TgId, err))
			continue
		}

		succeeded++
	}

	if len(errs) > 0 {
		return &DeliveryError{
			Succeeded: succeeded,
			Failed:    failed,
			Err:       errors.Join(errs...),
		}
	}
	return nil
}

// Close closes the underlying gRPC client connection if initialized.
func (c *CoreClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
