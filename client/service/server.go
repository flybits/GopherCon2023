package service

import (
	"context"
	"fmt"
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

	defer func() {
		if r := recover(); r != nil {
			//fmt.Printf("Recovered from panic at offset %v. Error: %v \n", offset, r)
			go func() {
				//request.Request
				//	offset++
				//fmt.Printf("will continue processing data from offset %d\n\n", offset)
				//	doMagic(offset, ch)
			}()
		}
	}()

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

		err = processData(data)
		if err != nil {
			log.Printf("error when processing data: %v", err)
			continue
		}

		log.Printf("processed data %v", data)
	}
	return nil
}

func processData(data *pb.Data) error {
	if data.UserID == "userID6" {
		return fmt.Errorf("mock error")
	}

	if data.UserID == "userID7" {
		panic("oops panic")
	}

	return nil
}
