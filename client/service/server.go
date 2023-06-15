package service

import (
	"context"
	"github.com/flybits/gophercon2023/server/pb"
	"google.golang.org/grpc"
	"io"
	"log"
)

type (
	ServerManager interface {
		//Close() error
		GetStreamFromServer(ctx context.Context, request *pb.DataRequest) error
	}

	server struct {
		connection *grpc.ClientConn
		grpcClient pb.ServerClient
	}
)

func NewServerManager(address string) (ServerManager, error) {
	log.Printf("Connecting to ctxrepo at %s", address)

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

func (s *server) GetStreamFromServer(ctx context.Context, request *pb.DataRequest) error {
	stream, err := s.grpcClient.GetData(ctx, request)
	if err != nil {
		return err
	}

	for {
		data, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Printf("Err var returned from stream Recv: %v", err)
			//return c.ShutDownEvaluationForAllWithStreamingGracefully(ctx, ru.ID, lastSuccessfullyProcessedUserID, lastSuccessfullyProcessedDeviceID, fireAndForget, origin, uniqueID, p, true)
		}

		log.Printf("received data: %v", data)
	}
	return nil
}
