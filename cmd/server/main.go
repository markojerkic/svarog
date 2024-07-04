package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"

	envParser "github.com/caarlos0/env/v11"
	dotenv "github.com/joho/godotenv"
	"github.com/markojerkic/svarog/internal/server/http"
	"github.com/markojerkic/svarog/internal/server/db"
	rpc "github.com/markojerkic/svarog/internal/proto"
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

var _ rpc.LoggAggregatorServer = &ImplementedServer{}

var logs = make(chan *rpc.LogLine, 1024*1024)

func (i *ImplementedServer) BatchLog(_ context.Context, batchLogs *rpc.Backlog) (*rpc.Void, error) {
	slog.Debug("Received batch log of size: ", slog.Int64("size", int64(len(batchLogs.Logs))))

	for _, logLine := range batchLogs.Logs {
		logs <- logLine
	}
	return &rpc.Void{}, nil
}

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

func loadEnv() Env {
	env := Env{}
	err := dotenv.Load()

	if err != nil {
		log.Fatalf("Error loading .env file. %v", err)
	}

	if err := envParser.Parse(&env); err != nil {
		log.Fatalf("Error parsing env: %+v\n", err)
	}

	return env
}

func main() {
	logOpts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(os.Stdout, logOpts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	env := loadEnv()

	grpcServerPort := env.GrpcServerPort

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", grpcServerPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	rpc.RegisterLoggAggregatorServer(grpcServer, newServer())

	mongoRepository := db.NewMongoClient(env.MongoUrl)

	mongoServer := db.NewLogServer(context.Background(), mongoRepository)

	httpServer := http.NewServer(mongoRepository)

	slog.Info(fmt.Sprintf("Starting gRPC server on port %d\n", grpcServerPort))
	go mongoServer.Run(logs)
	go httpServer.Start(env.HttpServerPort)
	grpcServer.Serve(lis)
}
