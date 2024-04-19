package main

import (
	"fmt"
	"log"
	"net"

	rpc "github.com/markojerkic/svarog/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type ImplementedServer struct {
	rpc.UnimplementedLoggAggregatorServer
}

var logs []*rpc.LogLine

// Log implements rpc.LoggAggregatorServer.
func (i *ImplementedServer) Log(stream rpc.LoggAggregator_LogServer) error {
	fmt.Println("Log called")

	for {
		logLine, err := stream.Recv()
		if err != nil {
			fmt.Printf("Failed to receive a log line: %v", err)
			return err
		}
		peer, ok := peer.FromContext(stream.Context())
		if !ok {
			return err
		}
		fmt.Printf("Received log line: %v from %v\n", logLine, peer.Addr)

		logs = append(logs, logLine)

		fmt.Printf("Received log line: %v\n", logLine)
	}
}

// mustEmbedUnimplementedLoggAggregatorServer implements rpc.LoggAggregatorServer.
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

	grpcServer.Serve(lis)

}
