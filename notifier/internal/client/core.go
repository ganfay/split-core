package client

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	pb "github.com/ganfay/split-proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CoreClient struct {
	client pb.NotificationServiceClient
	conn   *grpc.ClientConn
}

func NewCoreClient(target string) (*CoreClient, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("client: failed to connect to gRPC server: %w", err)
	}

	cl := pb.NewNotificationServiceClient(conn)

	return &CoreClient{client: cl, conn: conn}, nil
}

func (c *CoreClient) GetTargets(ctx context.Context, fundID int64, creatorID int64) (*pb.TargetsResponse, error) {
	targets, err := c.client.GetNotificationTargets(ctx, &pb.GetRequest{FundId: fundID, CreatorIid: creatorID})
	if err != nil {
		return nil, err
	}
	log.Printf("Core: Get targets successfully on gRPC connection")
	return targets, err
}

func (c *CoreClient) Ping(ctx context.Context, name string) (*pb.PingReply, error) {
	ping, err := c.client.SayPing(ctx, &pb.PingRequest{Name: name})
	return ping, err
}

func (c *CoreClient) NotificateTargets(ctx context.Context, targets []*pb.Target, amount float64, creatorName string, fundName string) error {
	for _, t := range targets {
		text := fmt.Sprintf(
			"🔔 <b>New expense in fund!</b>\n\n"+
				"Hello, <b>%s</b>! 👋\n\n"+
				"User <b>%s</b> right now added an expense in the amount of <code>%.2f</code> in fund <b>«%s»</b>. 💳",
			t.FirstName,
			creatorName,
			amount,
			fundName,
		)

		_, err := c.client.NotificateTarget(ctx, &pb.NotTarget{Msg: text, TgId: t.TgId})
		if err != nil {
			slog.Error("failed to send notification", "user", t.FirstName, "err", err)
			return err
		}
	}
	return nil
}

func (c *CoreClient) Close() error {
	return c.conn.Close()
}
