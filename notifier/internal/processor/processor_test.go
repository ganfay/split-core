package processor

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ganfay/split-notify/internal/client"
	pb "github.com/ganfay/split-proto/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockClient struct {
	getTargetsFn        func(ctx context.Context, fundID, creatorID int64) (*pb.TargetsResponse, error)
	notificateTargetsFn func(ctx context.Context, targets []*pb.Target, amount float64, creatorName, fundName string) error
}

func (m *mockClient) GetTargets(ctx context.Context, fundID, creatorID int64) (*pb.TargetsResponse, error) {
	if m.getTargetsFn != nil {
		return m.getTargetsFn(ctx, fundID, creatorID)
	}
	return &pb.TargetsResponse{}, nil
}

func (m *mockClient) NotificateTargets(ctx context.Context, targets []*pb.Target, amount float64, creatorName, fundName string) error {
	if m.notificateTargetsFn != nil {
		return m.notificateTargetsFn(ctx, targets, amount, creatorName, fundName)
	}
	return nil
}

func hasPermanentFlag(err error) bool {
	var p interface{ Permanent() bool }
	return errors.As(err, &p) && p.Permanent()
}

func TestHandleMessageValidationAndTypes(t *testing.T) {
	proc := NewProcessor(&mockClient{})

	tests := []struct {
		name          string
		eventType     string
		body          []byte
		wantErr       bool
		wantPermanent bool
	}{
		{
			name:          "unsupported event type is permanent",
			eventType:     "fund_deleted",
			body:          []byte(`{"fund_id":1,"creator_id":2,"amount":10}`),
			wantErr:       true,
			wantPermanent: true,
		},
		{
			name:          "empty event type is permanent",
			eventType:     "",
			body:          []byte(`{"fund_id":1,"creator_id":2,"amount":10}`),
			wantErr:       true,
			wantPermanent: true,
		},
		{
			name:          "malformed json is permanent",
			eventType:     "expense_created",
			body:          []byte(`{bad json`),
			wantErr:       true,
			wantPermanent: true,
		},
		{
			name:          "zero fund_id is permanent",
			eventType:     "expense_created",
			body:          []byte(`{"fund_id":0,"creator_id":2,"amount":10}`),
			wantErr:       true,
			wantPermanent: true,
		},
		{
			name:          "negative creator_id is permanent",
			eventType:     "expense_created",
			body:          []byte(`{"fund_id":1,"creator_id":-3,"amount":10}`),
			wantErr:       true,
			wantPermanent: true,
		},
		{
			name:          "non-positive amount is permanent",
			eventType:     "expense_created",
			body:          []byte(`{"fund_id":1,"creator_id":2,"amount":0}`),
			wantErr:       true,
			wantPermanent: true,
		},
		{
			name:      "valid event succeeds",
			eventType: "expense_created",
			body:      []byte(`{"fund_id":10,"creator_id":20,"amount":99.95}`),
			wantErr:   false,
		},
		{
			name:      "valid event with extra forward-compatible fields succeeds",
			eventType: "expense_created",
			body:      []byte(`{"fund_id":10,"creator_id":20,"amount":99.95,"currency":"USD"}`),
			wantErr:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := proc.HandleMessage(context.Background(), tc.body, tc.eventType)
			if (err != nil) != tc.wantErr {
				t.Fatalf("HandleMessage() err = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr && hasPermanentFlag(err) != tc.wantPermanent {
				t.Fatalf("hasPermanentFlag(%v) = %v, want %v", err, hasPermanentFlag(err), tc.wantPermanent)
			}
		})
	}

	t.Run("nil processor and NaN amount guarded", func(t *testing.T) {
		var nilProc *Processor
		if err := nilProc.HandleMessage(context.Background(), []byte(`{}`), "expense_created"); err == nil {
			t.Fatal("expected error for nil processor")
		}
		if err := proc.NotifyTargets(context.Background(), ExpenseCreatedEvent{FundID: 1, CreatorID: 1, Amount: math.NaN()}); !hasPermanentFlag(err) {
			t.Fatalf("expected permanent error for NaN amount, got %v", err)
		}
	})
}

func TestNotifyTargetsErrorClassificationAndPartialRetry(t *testing.T) {
	t.Run("skips NotificateTargets when no targets returned", func(t *testing.T) {
		called := false
		proc := NewProcessor(&mockClient{
			getTargetsFn: func(_ context.Context, _, _ int64) (*pb.TargetsResponse, error) {
				return &pb.TargetsResponse{Targets: nil, FundName: "Trip", CreatorName: "Alex"}, nil
			},
			notificateTargetsFn: func(_ context.Context, _ []*pb.Target, _ float64, _, _ string) error {
				called = true
				return nil
			},
		})

		err := proc.HandleMessage(context.Background(), []byte(`{"fund_id":1,"creator_id":2,"amount":10}`), "expense_created")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if called {
			t.Fatal("NotificateTargets should not be called when targets slice is empty")
		}
	})

	t.Run("GetTargets transient vs permanent classification", func(t *testing.T) {
		unavailableProc := NewProcessor(&mockClient{
			getTargetsFn: func(_ context.Context, _, _ int64) (*pb.TargetsResponse, error) {
				return nil, status.Error(codes.Unavailable, "bot starting")
			},
		})
		err := unavailableProc.HandleMessage(context.Background(), []byte(`{"fund_id":1,"creator_id":2,"amount":10}`), "expense_created")
		if err == nil || hasPermanentFlag(err) {
			t.Fatalf("expected transient error for Unavailable, got %v (permanent=%v)", err, hasPermanentFlag(err))
		}

		notFoundProc := NewProcessor(&mockClient{
			getTargetsFn: func(_ context.Context, _, _ int64) (*pb.TargetsResponse, error) {
				return nil, status.Error(codes.NotFound, "fund missing")
			},
		})
		err = notFoundProc.HandleMessage(context.Background(), []byte(`{"fund_id":1,"creator_id":2,"amount":10}`), "expense_created")
		if !hasPermanentFlag(err) {
			t.Fatalf("expected permanent error for NotFound, got %v", err)
		}
	})

	t.Run("partial failure retries only failed targets and recovers without duplicating succeeded ones", func(t *testing.T) {
		var attempts atomic.Int32
		proc := NewProcessor(&mockClient{
			getTargetsFn: func(_ context.Context, _, _ int64) (*pb.TargetsResponse, error) {
				return &pb.TargetsResponse{
					Targets: []*pb.Target{
						{FirstName: "Alice", TgId: 101},
						{FirstName: "Bob", TgId: 202},
					},
					FundName:    "Trip",
					CreatorName: "Max",
				}, nil
			},
			notificateTargetsFn: func(_ context.Context, targets []*pb.Target, _ float64, _, _ string) error {
				call := attempts.Add(1)
				if call == 1 {
					if len(targets) != 2 {
						t.Fatalf("first call expected 2 targets, got %d", len(targets))
					}
					return &client.DeliveryError{
						Succeeded: 1,
						Failed:    []*pb.Target{{FirstName: "Bob", TgId: 202}},
						Err:       status.Error(codes.Unavailable, "blip"),
					}
				}
				if len(targets) != 1 || targets[0].GetTgId() != 202 {
					t.Fatalf("retry call expected only failed target 202, got %+v", targets)
				}
				return nil
			},
		}, WithPartialRetry(2, time.Millisecond))

		err := proc.HandleMessage(context.Background(), []byte(`{"fund_id":1,"creator_id":2,"amount":50}`), "expense_created")
		if err != nil {
			t.Fatalf("expected recovery on partial retry, got %v", err)
		}
		if attempts.Load() != 2 {
			t.Fatalf("attempts = %d, want 2", attempts.Load())
		}
	})

	t.Run("persistent partial failure becomes permanent so already-notified users are not spammed", func(t *testing.T) {
		proc := NewProcessor(&mockClient{
			getTargetsFn: func(_ context.Context, _, _ int64) (*pb.TargetsResponse, error) {
				return &pb.TargetsResponse{
					Targets: []*pb.Target{
						{FirstName: "Alice", TgId: 101},
						{FirstName: "Bob", TgId: 202},
					},
				}, nil
			},
			notificateTargetsFn: func(_ context.Context, targets []*pb.Target, _ float64, _, _ string) error {
				return &client.DeliveryError{
					Succeeded: 1,
					Failed:    targets[len(targets)-1:],
					Err:       status.Error(codes.Unknown, "bot blocked by user"),
				}
			},
		}, WithPartialRetry(1, time.Millisecond))

		err := proc.HandleMessage(context.Background(), []byte(`{"fund_id":1,"creator_id":2,"amount":50}`), "expense_created")
		if !hasPermanentFlag(err) {
			t.Fatalf("expected permanent error after partial failure with exhausted retries, got %v", err)
		}
	})

	t.Run("total failure with transient error stays transient for consumer retry", func(t *testing.T) {
		proc := NewProcessor(&mockClient{
			getTargetsFn: func(_ context.Context, _, _ int64) (*pb.TargetsResponse, error) {
				return &pb.TargetsResponse{
					Targets: []*pb.Target{{FirstName: "Alice", TgId: 101}},
				}, nil
			},
			notificateTargetsFn: func(_ context.Context, targets []*pb.Target, _ float64, _, _ string) error {
				return &client.DeliveryError{
					Succeeded: 0,
					Failed:    targets,
					Err:       status.Error(codes.Unavailable, "down"),
				}
			},
		})

		err := proc.HandleMessage(context.Background(), []byte(`{"fund_id":1,"creator_id":2,"amount":50}`), "expense_created")
		if err == nil || hasPermanentFlag(err) {
			t.Fatalf("expected transient error when 0 targets succeeded, got %v (permanent=%v)", err, hasPermanentFlag(err))
		}
	})

	t.Run("joined errors classify correctly when all permanent vs mixed", func(t *testing.T) {
		allPerm := NewProcessor(&mockClient{
			getTargetsFn: func(_ context.Context, _, _ int64) (*pb.TargetsResponse, error) {
				return &pb.TargetsResponse{
					Targets: []*pb.Target{{FirstName: "A", TgId: 1}, {FirstName: "B", TgId: 2}},
				}, nil
			},
			notificateTargetsFn: func(_ context.Context, targets []*pb.Target, _ float64, _, _ string) error {
				return &client.DeliveryError{
					Succeeded: 0,
					Failed:    targets,
					Err: errors.Join(
						status.Error(codes.InvalidArgument, "bad1"),
						status.Error(codes.NotFound, "bad2"),
					),
				}
			},
		})
		if err := allPerm.HandleMessage(context.Background(), []byte(`{"fund_id":1,"creator_id":2,"amount":50}`), "expense_created"); !hasPermanentFlag(err) {
			t.Fatalf("expected permanent error for all permanent joined errors, got %v", err)
		}

		mixed := NewProcessor(&mockClient{
			getTargetsFn: func(_ context.Context, _, _ int64) (*pb.TargetsResponse, error) {
				return &pb.TargetsResponse{
					Targets: []*pb.Target{{FirstName: "A", TgId: 1}, {FirstName: "B", TgId: 2}},
				}, nil
			},
			notificateTargetsFn: func(_ context.Context, targets []*pb.Target, _ float64, _, _ string) error {
				return &client.DeliveryError{
					Succeeded: 0,
					Failed:    targets,
					Err: errors.Join(
						status.Error(codes.InvalidArgument, "bad1"),
						status.Error(codes.Unavailable, "temp"),
					),
				}
			},
		})
		if err := mixed.HandleMessage(context.Background(), []byte(`{"fund_id":1,"creator_id":2,"amount":50}`), "expense_created"); err == nil || hasPermanentFlag(err) {
			t.Fatalf("expected transient error for mixed joined errors with 0 succeeded, got %v", err)
		}
	})

	t.Run("partial failure with permanent error aborts retries immediately", func(t *testing.T) {
		var calls atomic.Int32
		proc := NewProcessor(&mockClient{
			getTargetsFn: func(_ context.Context, _, _ int64) (*pb.TargetsResponse, error) {
				return &pb.TargetsResponse{
					Targets: []*pb.Target{{FirstName: "A", TgId: 1}, {FirstName: "B", TgId: 2}},
				}, nil
			},
			notificateTargetsFn: func(_ context.Context, targets []*pb.Target, _ float64, _, _ string) error {
				calls.Add(1)
				return &client.DeliveryError{
					Succeeded: 1,
					Failed:    targets[1:],
					Err:       status.Error(codes.InvalidArgument, "invalid chat"),
				}
			},
		}, WithPartialRetry(3, time.Millisecond))

		err := proc.HandleMessage(context.Background(), []byte(`{"fund_id":1,"creator_id":2,"amount":50}`), "expense_created")
		if !hasPermanentFlag(err) {
			t.Fatalf("expected permanent error, got %v", err)
		}
		if calls.Load() != 1 {
			t.Fatalf("expected 1 call without partial retry on permanent gRPC error, got %d", calls.Load())
		}
	})
}
