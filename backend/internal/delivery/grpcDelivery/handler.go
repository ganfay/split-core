package grpcDelivery

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ganfay/split-core/internal/domain"
	pb "github.com/ganfay/split-proto/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	tele "gopkg.in/telebot.v4"
)

const grpcOpTimeout = 5 * time.Second

type Handler struct {
	pb.UnimplementedNotificationServiceServer
	fundUC domain.FundUsecase
	bot    *tele.Bot
}

func NewHandler(fundUC domain.FundUsecase, b *tele.Bot) *Handler {
	return &Handler{fundUC: fundUC, bot: b}
}

func (h *Handler) SayPing(_ context.Context, in *pb.PingRequest) (*pb.PingReply, error) {
	return &pb.PingReply{Message: "Ping " + in.GetName()}, nil
}

func (h *Handler) GetNotificationTargets(ctx context.Context, in *pb.GetRequest) (*pb.TargetsResponse, error) {
	if in == nil || in.GetFundId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "fund_id must be positive")
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, grpcOpTimeout)
		defer cancel()
	}

	fundID := in.GetFundId()
	creatorIID := in.GetCreatorIid()
	reqFund := &domain.Fund{
		ID: int(fundID),
	}
	slog.Info("Received:", "fund_id", fundID, "creator_id", creatorIID)
	info, err := h.fundUC.GetInfo(ctx, reqFund)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "fund not found")
		}
		return nil, err
	}
	members, err := h.fundUC.GetMembers(ctx, int(fundID))
	if err != nil {
		slog.Error("Error to get members for notification service", "err", err)
		return nil, err
	}
	var creatorName string
	var targets []*pb.Target
	for _, m := range members {
		slog.Debug("TargetUser", "target", m)
		if m.IsVirtual {
			continue
		}
		if m.ID == creatorIID {
			creatorName = m.FirstName
			continue
		}
		if m.TgID == nil || *m.TgID <= 0 {
			continue
		}

		target := &pb.Target{
			Iid:       m.ID,
			FirstName: m.FirstName,
			TgId:      *m.TgID,
		}

		targets = append(targets, target)
	}
	return &pb.TargetsResponse{Targets: targets, FundName: info.Name, CreatorName: creatorName}, nil
}

func (h *Handler) NotificateTarget(_ context.Context, in *pb.NotTarget) (*pb.NotificateReply, error) {
	if in == nil || in.GetTgId() <= 0 {
		return &pb.NotificateReply{Success: false}, status.Error(codes.InvalidArgument, "tg_id must be positive")
	}
	if h.bot == nil {
		return &pb.NotificateReply{Success: false}, status.Error(codes.FailedPrecondition, "telegram bot is not initialized")
	}
	_, err := h.bot.Send(&tele.User{ID: in.GetTgId()}, in.GetMsg(), tele.ModeHTML)
	if err != nil {
		slog.Error("gRPC server: failed to send telegram message", "err", err)
		return &pb.NotificateReply{Success: false}, err
	}
	return &pb.NotificateReply{Success: true}, nil
}
