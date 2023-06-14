package service

import (
	"context"
	"github.com/flybits/gophercon2023/server/handler"
	"google.golang.org/grpc"
)

type (
	ServerManager interface {
		//Close() error
		GetStreamFromServer(ctx context.Context, request handler.Handler)
	}

	server struct {
		connection *grpc.ClientConn
		//	grpcClient ctxrepoPB.ContextRepoClient
	}
)

//func (s *server)
