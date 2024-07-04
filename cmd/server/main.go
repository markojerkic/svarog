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
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type Env struct {
	MongoUrl                 string   `env:"MONGO_URL"`
	GrpcServerPort           int      `env:"GPRC_PORT"`
	HttpServerPort           int      `env:"HTTP_SERVER_PORT"`
	HttpServerAllowedOrigins []string `env:"HTTP_SERVER_ALLOWED_ORIGINS"`
}

type ImplementedServer struct {
	rpc.UnimplementedLoggAggregatorServer
}

var _ rpc.LoggAggregatorServer = &ImplementedServer{}

var logIngestChannel = make(chan *rpc.LogLine, 1024*1024)

func (i *ImplementedServer) BatchLog(_ context.Context, batchLogs *rpc.Backlog) (*rpc.Void, error) {
	slog.Debug("Received batch log of size: ", slog.Int64("size", int64(len(batchLogs.Logs))))

	for _, logLine := range batchLogs.Logs {
		logIngestChannel <- logLine
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

		logIngestChannel <- logLine
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

func startGrpcServer(env Env) {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", env.GrpcServerPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	rpc.RegisterLoggAggregatorServer(grpcServer, newServer())

	grpcServer.Serve(lis)
}

func setupLogger() {
	logOpts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(os.Stdout, logOpts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func main() {
	setupLogger()
	env := loadEnv()

	mongoRepository := db.NewMongoClient(env.MongoUrl)
	logServer := db.NewLogServer(context.Background(), mongoRepository)
	httpServer := http.NewServer(mongoRepository, http.HttpServerOptions{
		AllowedOrigins: env.HttpServerAllowedOrigins,
		ServerPort:     env.HttpServerPort,
	})

	slog.Info(fmt.Sprintf("Starting gRPC server on port %d, HTTP server on port %d\n", env.GrpcServerPort, env.HttpServerPort))

	go logServer.Run(logIngestChannel)
	go httpServer.Start()
	startGrpcServer(env)
}
