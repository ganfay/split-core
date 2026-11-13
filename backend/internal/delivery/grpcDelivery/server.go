package grpcDelivery

import (
	"net"
	"time"

	"github.com/ganfay/split-core/internal/domain"
	pb "github.com/ganfay/split-proto/pb"
	"google.golang.org/grpc"
	tele "gopkg.in/telebot.v4"
)

type Server struct {
	addr       string
	grpcServer *grpc.Server
}

func NewServer(addr string, fundUC domain.FundUsecase, b *tele.Bot) *Server {
	s := grpc.NewServer()
	pb.RegisterNotificationServiceServer(s, NewHandler(fundUC, b))
	return &Server{addr: addr, grpcServer: s}
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		s.grpcServer.Stop()
	}
}
