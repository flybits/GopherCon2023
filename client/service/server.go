package service

import (
	"context"
	"github.com/flybits/gophercon2023/server/pb"
	"google.golang.org/grpc"
)

type (
	ServerManager interface {
		//Close() error
		GetStreamFromServer(ctx context.Context, request pb.DataRequest) error
	}

	server struct {
		connection *grpc.ClientConn
		grpcClient pb.ServerClient
	}
)

func (s *server) GetStreamFromServer(ctx context.Context, request *pb.DataRequest) error {
	sc, err := s.grpcClient.GetData(ctx, request)
	if err != nil {
		return err
	}
	sc.Recv()
	return nil
}
