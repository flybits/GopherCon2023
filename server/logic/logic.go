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
	for i := 0; i < 1000; i++ {
		if err := stream.Send(&pb.Data{
			UserID:   fmt.Sprintf("userID%v", i),
			TenantID: fmt.Sprintf("tenantID%v", i),
			DeviceID: fmt.Sprintf("deviceID%v", i),
			Name:     fmt.Sprintf("name%v", i),
		}); err != nil {
			log.Printf("error streaming data: %s", err.Error())
			return err
		}
	}

	return nil
}
