package main

import (
	"fmt"
	"log"
	"net"

	rpc "github.com/markojerkic/svarog/proto"
	"github.com/markojerkic/svarog/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type ImplementedServer struct {
	rpc.UnimplementedLoggAggregatorServer
}

var logs = make(chan *rpc.LogLine, 1024*1024)

// Log implements rpc.LoggAggregatorServer.
func (i *ImplementedServer) Log(stream rpc.LoggAggregator_LogServer) error {
	for {
		logLine, err := stream.Recv()
		if err != nil {
			return err
		}
		_, ok := peer.FromContext(stream.Context())
		if !ok {
			return err
		}
        logs <- logLine
	}
}

func (i *ImplementedServer) mustEmbedUnimplementedLoggAggregatorServer() {}

func newServer() rpc.LoggAggregatorServer {
	return &ImplementedServer{}
}

func main() {
	println("Hello, World!")

	port := 50051

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	rpc.RegisterLoggAggregatorServer(grpcServer, newServer())

    mongoServer := db.NewLogServer()

    go mongoServer.Run(logs)

	grpcServer.Serve(lis)

}
