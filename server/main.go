package main

import (
	"fmt"
	"log"
	"net"

	envParser "github.com/caarlos0/env/v11"
	"github.com/markojerkic/svarog/db"
	rpc "github.com/markojerkic/svarog/proto"
	"github.com/markojerkic/svarog/server/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type Env struct {
	MongoUrl       string `env:"MONGO_URL"`
	GrpcServerPort int    `env:"GPRC_PORT"`
	HttpServerPort int    `env:"HTTP_SERVER_PORT"`
}

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
	env := Env{}

	if err := envParser.Parse(env); err != nil {
		log.Fatalf("%+v\n", err)
	}

	grpcServerPort := env.GrpcServerPort

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", grpcServerPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	rpc.RegisterLoggAggregatorServer(grpcServer, newServer())

	mongoRepository := db.NewMongoClient(env.MongoUrl)
	mongoServer := db.NewLogServer(mongoRepository)

	httpServer := http.NewServer(mongoRepository)

	log.Printf("Starting gRPC server on port %d\n", grpcServerPort)
	go mongoServer.Run(logs)
	go httpServer.Start(env.HttpServerPort)
	grpcServer.Serve(lis)
}
