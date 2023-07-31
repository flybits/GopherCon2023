package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/flybits/gophercon2023/amqp"
	"github.com/flybits/gophercon2023/server/pb"
	"google.golang.org/grpc"
	"io"
	"log"
)

type (
	ServerManager interface {
		//Close() error
		GetStreamFromServer(ctx context.Context, offset int32) error
	}

	server struct {
		connection *grpc.ClientConn
		grpcClient pb.ServerClient
		broker     *amqp.Broker
	}
)

func NewServerManager(address string, broker *amqp.Broker) (ServerManager, error) {
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
		broker:     broker,
	}, nil
}

func (s *server) GetStreamFromServer(ctx context.Context, offset int32) error {

	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from panic at offset %v: panic %v", offset, r)

			go func() {
				offset++
				log.Printf("will continue processing data from offset %d\n", offset)
				s.GetStreamFromServer(ctx, offset)
			}()
		}
	}()

	// send a sample rabbitmq message for testing
	bubu := struct {
		Bubu string `json:"bubu"`
	}{
		Bubu: "arman",
	}
	b, err := json.Marshal(bubu)
	if err != nil {
		log.Printf("error happened when marshaling: %v", err)
	}
	publish := amqp.PublishWithDefaults("client", "routingKey", b)
	err = s.broker.Publish(ctx, publish)

	if err != nil {
		log.Printf("error when publishing: %v", err)
	}

	// make request for streaming
	request := &pb.DataRequest{
		Offset: offset,
	}

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
	if data.Value == 6 {
		return fmt.Errorf("mock error")
	}

	if data.Value == 7 {
		panic("oops panic")
	}

	return nil
}
