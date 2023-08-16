package service

import (
	"context"
	"github.com/flybits/gophercon2023/server/pb"
	"google.golang.org/grpc"
	"log"
)

type (
	ServerManager interface {
		//Close() error
		GetStreamFromServer(ctx context.Context, offset int32) (pb.Server_GetDataClient, error)
	}

	server struct {
		connection *grpc.ClientConn
		grpcClient pb.ServerClient
	}
)

func NewServerManager(address string) (ServerManager, error) {
	log.Printf("Connecting to server at %s", address)

	conn, err := grpc.Dial(
		address,
		grpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	return &server{
		connection: conn,
		grpcClient: pb.NewServerClient(conn),
	}, nil
}

func (s *server) GetStreamFromServer(ctx context.Context, offset int32) (pb.Server_GetDataClient, error) {

	request := &pb.DataRequest{
		Offset: offset,
	}

	stream, err := s.grpcClient.GetData(ctx, request)
	if err != nil {
		return nil, err
	}

	return stream, nil
}
