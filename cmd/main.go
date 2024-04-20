package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	dotenv "github.com/joho/godotenv"
	"github.com/markojerkic/svarog/db"
	rpc "github.com/markojerkic/svarog/proto"
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
	err := dotenv.Load()

	if err != nil {
		log.Fatalf("Error loading .env file. %v", err)
	}

	grpcServerPortEnv, ok := os.LookupEnv("GPRC_PORT")
	if !ok {
		log.Fatalf("GRPC_PORT is not set.")
	}

	grpcServerPort, err := strconv.Atoi(grpcServerPortEnv)
	if err != nil {
		log.Fatalf("Error converting GRPC_PORT to int. %v", err)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", grpcServerPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	rpc.RegisterLoggAggregatorServer(grpcServer, newServer())

	mongoUrl, ok := os.LookupEnv("MONGO_URL")
	if !ok {
		log.Fatal("MONGO_URL is not set")
	}

	mongoServer := db.NewLogServer(db.NewMongoClient(mongoUrl))

	go mongoServer.Run(logs)

	log.Printf("Starting gRPC server on port %d\n", grpcServerPort)
	grpcServer.Serve(lis)
}
