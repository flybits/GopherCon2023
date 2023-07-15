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

	offset := req.Offset
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from panic at offset %v: panic %v", offset, r)

			go func() {
				offset++
				req.Offset = offset
				log.Printf("will continue sending data from offset %d\n", offset)
				s.GetData(req, stream)
			}()
		}
	}()

	for offset = req.Offset; offset < 10; offset++ {

		d, err := retrieveData(offset)
		if err != nil {
			log.Printf("error when retrieving data: %v", err)
		}

		if err := stream.Send(d); err != nil {
			log.Printf("error streaming data: %s", err.Error())
			return err
		}
	}

	return nil
}

func retrieveData(i int32) (*pb.Data, error) {

	if i == 5 {
		return nil, fmt.Errorf("some error happened")
	}

	if i == 8 {
		panic("oops panic on server")
	}

	return &pb.Data{
		UserID: fmt.Sprintf("userID%v", i),
		Value:  i,
	}, nil
}