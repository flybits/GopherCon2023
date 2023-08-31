package logic

import (
	"fmt"
	"github.com/flybits/gophercon2023/server/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"log"
	"net"
)

type (
	//Server is a struct for gRPC server
	Server struct {
		pb.UnimplementedServerServer
	}
)

// Setup registers and starts an rpc server
func Setup(port string) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.KeepaliveParams(
			keepalive.ServerParameters{
				//MaxConnectionIdle: 5 * time.Minute,
			},
		),
	)

	server := Server{}

	pb.RegisterServerServer(s, &server)

	go func() {
		log.Printf("starting grpc server...")
		if err := s.Serve(lis); err != nil {
			log.Printf("grpc server error :: %s", err)
		}
	}()
	return s, nil
}

// GetData streams the data back to the client
func (s *Server) GetData(req *pb.DataRequest, stream pb.Server_GetDataServer) error {

	var i int32
	log.Printf("received request to get data %v", req)

	for i = 0; i < 20; i++ {

		d, err := retrieveData(i)
		if err != nil {
			log.Printf("error when retrieving data: %v", err)
			continue
		}

		log.Printf("sending data %v", d)
		if err := stream.Send(d); err != nil {
			log.Printf("error streaming data: %s", err.Error())
			return err
		}
		log.Printf("data sent %v", d)
	}

	return nil
}

func retrieveData(i int32) (*pb.Data, error) {
	return &pb.Data{
		UserID: fmt.Sprintf("userID%v", i),
		Value:  i,
	}, nil
}
