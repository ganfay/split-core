package client

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/ganfay/split-proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockNotificationClient struct {
	mu           sync.Mutex
	getTargets   func(ctx context.Context, in *pb.GetRequest) (*pb.TargetsResponse, error)
	notificate   func(ctx context.Context, in *pb.NotTarget) (*pb.NotificateReply, error)
	sayPing      func(ctx context.Context, in *pb.PingRequest) (*pb.PingReply, error)
	sentPayloads []*pb.NotTarget
}

func (m *mockNotificationClient) SayPing(ctx context.Context, in *pb.PingRequest, _ ...grpc.CallOption) (*pb.PingReply, error) {
	if m.sayPing != nil {
		return m.sayPing(ctx, in)
	}
	return &pb.PingReply{Message: "Ping " + in.GetName()}, nil
}

func (m *mockNotificationClient) GetNotificationTargets(ctx context.Context, in *pb.GetRequest, _ ...grpc.CallOption) (*pb.TargetsResponse, error) {
	if m.getTargets != nil {
		return m.getTargets(ctx, in)
	}
	return &pb.TargetsResponse{}, nil
}

func (m *mockNotificationClient) NotificateTarget(ctx context.Context, in *pb.NotTarget, _ ...grpc.CallOption) (*pb.NotificateReply, error) {
	m.mu.Lock()
	m.sentPayloads = append(m.sentPayloads, in)
	m.mu.Unlock()
	if m.notificate != nil {
		return m.notificate(ctx, in)
	}
	return &pb.NotificateReply{Success: true}, nil
}

func TestFormatExpenseNotificationHTMLEscaping(t *testing.T) {
	tests := []struct {
		name        string
		firstName   string
		creatorName string
		amount      float64
		fundName    string
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "escapes html tags and ampersands in user fields without breaking template tags",
			firstName:   `<script>alert("xss")</script>`,
			creatorName: `Alice & Bob`,
			amount:      42.5,
			fundName:    `<b>Drinks</b> & 'Snacks'`,
			wantContain: []string{
				`<b>New expense in fund!</b>`,
				`Hello, <b>&lt;script&gt;alert(&#34;xss&#34;)&lt;/script&gt;</b>!`,
				`User <b>Alice &amp; Bob</b> right now added an expense in the amount of <code>42.50</code> in fund <b>«&lt;b&gt;Drinks&lt;/b&gt; &amp; &#39;Snacks&#39;»</b>.`,
			},
			wantAbsent: []string{
				`<script>`,
				`&amp;amp;`,
				`&amp;lt;`,
				`&lt;b&gt;New expense`,
			},
		},
		{
			name:        "does not double escape existing literal entities",
			firstName:   `Tom & Jerry`,
			creatorName: `O'Reilly`,
			amount:      10,
			fundName:    `A < B > C`,
			wantContain: []string{
				`<b>Tom &amp; Jerry</b>`,
				`<b>O&#39;Reilly</b>`,
				`<b>«A &lt; B &gt; C»</b>`,
			},
			wantAbsent: []string{
				`&amp;amp;`,
			},
		},
		{
			name:        "empty fields use safe defaults",
			firstName:   "   ",
			creatorName: "",
			amount:      1.234,
			fundName:    "",
			wantContain: []string{
				`Hello, <b>there</b>!`,
				`User <b>Unknown</b>`,
				`<code>1.23</code>`,
				`<b>«Untitled»</b>`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatExpenseNotification(tc.firstName, tc.creatorName, tc.amount, tc.fundName)
			for _, sub := range tc.wantContain {
				if !strings.Contains(got, sub) {
					t.Errorf("formatted message missing %q\ngot:\n%s", sub, got)
				}
			}
			for _, bad := range tc.wantAbsent {
				if strings.Contains(got, bad) {
					t.Errorf("formatted message unexpectedly contains %q\ngot:\n%s", bad, got)
				}
			}
		})
	}
}

func TestNotificateTargetsEdgeCases(t *testing.T) {
	t.Run("skips nil and zero tg_id targets and continues after single target failure", func(t *testing.T) {
		mock := &mockNotificationClient{
			notificate: func(_ context.Context, in *pb.NotTarget) (*pb.NotificateReply, error) {
				if in.GetTgId() == 200 {
					return &pb.NotificateReply{Success: false}, status.Error(codes.Unavailable, "temporary error")
				}
				return &pb.NotificateReply{Success: true}, nil
			},
		}
		c, err := NewCoreClientWithService(mock)
		if err != nil {
			t.Fatalf("NewCoreClientWithService: %v", err)
		}

		targets := []*pb.Target{
			nil,
			{Iid: 1, FirstName: "NoTG", TgId: 0},
			{Iid: 2, FirstName: "NegativeTG", TgId: -5},
			{Iid: 3, FirstName: "Alice", TgId: 100},
			{Iid: 4, FirstName: "Bob", TgId: 200},
			{Iid: 5, FirstName: "Charlie", TgId: 300},
		}

		err = c.NotificateTargets(context.Background(), targets, 25.0, "Dave", "Trip")
		var delErr *DeliveryError
		if !errors.As(err, &delErr) {
			t.Fatalf("expected *DeliveryError, got %v", err)
		}
		if delErr.Succeeded != 2 {
			t.Fatalf("Succeeded = %d, want 2", delErr.Succeeded)
		}
		if len(delErr.Failed) != 1 || delErr.Failed[0].GetTgId() != 200 {
			t.Fatalf("Failed = %+v, want [tg_id=200]", delErr.Failed)
		}
		if len(mock.sentPayloads) != 3 {
			t.Fatalf("sentPayloads = %d, want 3 (skipped nil/invalid targets and continued past failure)", len(mock.sentPayloads))
		}
	})

	t.Run("treats Success=false with nil error as failure", func(t *testing.T) {
		mock := &mockNotificationClient{
			notificate: func(_ context.Context, _ *pb.NotTarget) (*pb.NotificateReply, error) {
				return &pb.NotificateReply{Success: false}, nil
			},
		}
		c, _ := NewCoreClientWithService(mock)
		err := c.NotificateTargets(context.Background(), []*pb.Target{{FirstName: "Alice", TgId: 100}}, 10, "Bob", "Food")
		if err == nil {
			t.Fatal("expected error when reply.Success is false")
		}
	})

	t.Run("applies per-RPC timeout when caller ctx has no deadline", func(t *testing.T) {
		mock := &mockNotificationClient{
			notificate: func(ctx context.Context, _ *pb.NotTarget) (*pb.NotificateReply, error) {
				if _, ok := ctx.Deadline(); !ok {
					t.Error("expected RPC context to have deadline")
				}
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
		c, _ := NewCoreClientWithService(mock, WithRPCTimeout(30*time.Millisecond))

		err := c.NotificateTargets(context.Background(), []*pb.Target{{FirstName: "Alice", TgId: 100}}, 10, "Bob", "Food")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got %v", err)
		}
	})

	t.Run("stops immediately on canceled parent context and lists remaining targets", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		mock := &mockNotificationClient{
			notificate: func(_ context.Context, in *pb.NotTarget) (*pb.NotificateReply, error) {
				if in.GetTgId() == 100 {
					cancel()
				}
				return &pb.NotificateReply{Success: true}, nil
			},
		}
		c, _ := NewCoreClientWithService(mock)
		targets := []*pb.Target{
			{FirstName: "A", TgId: 100},
			{FirstName: "B", TgId: 200},
			{FirstName: "C", TgId: 300},
		}

		err := c.NotificateTargets(ctx, targets, 10, "Bob", "Food")
		var delErr *DeliveryError
		if !errors.As(err, &delErr) {
			t.Fatalf("expected *DeliveryError, got %v", err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
		if delErr.Succeeded != 1 || len(delErr.Failed) != 2 {
			t.Fatalf("Succeeded=%d Failed=%d, want 1 and 2", delErr.Succeeded, len(delErr.Failed))
		}
	})
}

func TestGetTargetsAndConstructorsAndClose(t *testing.T) {
	t.Run("nil client guards", func(t *testing.T) {
		var nilClient *CoreClient
		if err := nilClient.Close(); err != nil {
			t.Fatalf("Close on nil client returned error: %v", err)
		}
		if _, err := nilClient.GetTargets(context.Background(), 1, 1); err == nil {
			t.Fatal("GetTargets on nil client expected error")
		}
		if _, err := nilClient.Ping(context.Background(), "x"); err == nil {
			t.Fatal("Ping on nil client expected error")
		}
		if err := nilClient.NotificateTargets(context.Background(), nil, 1, "a", "b"); err == nil {
			t.Fatal("NotificateTargets on nil client expected error")
		}
	})

	t.Run("empty target rejected", func(t *testing.T) {
		if _, err := NewCoreClient("   "); err == nil {
			t.Fatal("expected error for empty target")
		}
		if _, err := NewCoreClientWithService(nil); err == nil {
			t.Fatal("expected error for nil service")
		}
	})

	t.Run("GetTargets nil response returns error", func(t *testing.T) {
		mock := &mockNotificationClient{
			getTargets: func(_ context.Context, _ *pb.GetRequest) (*pb.TargetsResponse, error) {
				return nil, nil
			},
		}
		c, _ := NewCoreClientWithService(mock)
		if _, err := c.GetTargets(context.Background(), 1, 2); err == nil {
			t.Fatal("expected error on nil response")
		}
	})

	t.Run("GetTargets enforces timeout against hanging gRPC server", func(t *testing.T) {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen: %v", err)
		}
		srv := grpc.NewServer()
		pb.RegisterNotificationServiceServer(srv, &hangingServer{})
		go func() { _ = srv.Serve(lis) }()
		defer srv.Stop()

		c, err := NewCoreClient(lis.Addr().String(), WithRPCTimeout(50*time.Millisecond))
		if err != nil {
			t.Fatalf("NewCoreClient: %v", err)
		}
		defer func() { _ = c.Close() }()

		_, err = c.GetTargets(context.Background(), 1, 2)
		if status.Code(err) != codes.DeadlineExceeded && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected DeadlineExceeded, got %v", err)
		}
	})
}

type hangingServer struct {
	pb.UnimplementedNotificationServiceServer
}

func (s *hangingServer) GetNotificationTargets(ctx context.Context, _ *pb.GetRequest) (*pb.TargetsResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type testGRPCServer struct {
	pb.UnimplementedNotificationServiceServer
	mu       sync.Mutex
	received []*pb.NotTarget
}

func (s *testGRPCServer) SayPing(_ context.Context, in *pb.PingRequest) (*pb.PingReply, error) {
	return &pb.PingReply{Message: "Ping " + in.GetName()}, nil
}

func (s *testGRPCServer) GetNotificationTargets(_ context.Context, in *pb.GetRequest) (*pb.TargetsResponse, error) {
	return &pb.TargetsResponse{
		Targets: []*pb.Target{
			{Iid: 2, FirstName: "<b>Bob</b>", TgId: 777},
		},
		FundName:    "Food & Drinks",
		CreatorName: "Alice <Admin>",
	}, nil
}

func (s *testGRPCServer) NotificateTarget(_ context.Context, in *pb.NotTarget) (*pb.NotificateReply, error) {
	s.mu.Lock()
	s.received = append(s.received, in)
	s.mu.Unlock()
	return &pb.NotificateReply{Success: true}, nil
}

func TestCoreClientRealGRPCRoundTrip(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	grpcSrv := grpc.NewServer()
	impl := &testGRPCServer{}
	pb.RegisterNotificationServiceServer(grpcSrv, impl)
	go func() { _ = grpcSrv.Serve(lis) }()
	defer grpcSrv.Stop()

	c, err := NewCoreClient(lis.Addr().String(), WithRPCTimeout(2*time.Second))
	if err != nil {
		t.Fatalf("NewCoreClient: %v", err)
	}
	defer func() { _ = c.Close() }()

	ping, err := c.Ping(context.Background(), "notifier")
	if err != nil || ping.GetMessage() != "Ping notifier" {
		t.Fatalf("Ping() = (%v, %v), want Ping notifier", ping, err)
	}

	resp, err := c.GetTargets(context.Background(), 10, 1)
	if err != nil {
		t.Fatalf("GetTargets: %v", err)
	}

	if err := c.NotificateTargets(context.Background(), resp.GetTargets(), 19.5, resp.GetCreatorName(), resp.GetFundName()); err != nil {
		t.Fatalf("NotificateTargets: %v", err)
	}

	impl.mu.Lock()
	defer impl.mu.Unlock()
	if len(impl.received) != 1 {
		t.Fatalf("received %d RPCs, want 1", len(impl.received))
	}
	msg := impl.received[0].GetMsg()
	if !strings.Contains(msg, "&lt;b&gt;Bob&lt;/b&gt;") ||
		!strings.Contains(msg, "Alice &lt;Admin&gt;") ||
		!strings.Contains(msg, "Food &amp; Drinks") {
		t.Fatalf("unexpected escaped payload over gRPC:\n%s", msg)
	}
}
