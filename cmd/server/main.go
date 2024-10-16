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

var logIngestChannel = make(chan db.LogLineWithIp, 1024*1024)

func getIp(ctx context.Context) (string, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("failed to get peer from context")
	}

	// parse out ipv4 address from peer
	ipv6, _, err := net.SplitHostPort(peer.Addr.String())
	if err != nil {
		return "", err
	}
	ip := net.ParseIP(ipv6)

	var ipv4 string
	if ip.IsLoopback() {
		ipv4 = "127.0.0.1"
	} else {
		ipv4 = ip.To4().String()
	}

	return ipv4, nil
}

func (i *ImplementedServer) BatchLog(ctx context.Context, batchLogs *rpc.Backlog) (*rpc.Void, error) {
	ipv4, err := getIp(ctx)
	if err != nil {
		return &rpc.Void{}, err
	}
	slog.Debug("Received batch log", slog.Int64("size", int64(len(batchLogs.Logs))), slog.String("ip", ipv4))

	for _, logLine := range batchLogs.Logs {
		logIngestChannel <- db.LogLineWithIp{LogLine: logLine, Ip: ipv4}
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
		ipv4, err := getIp(stream.Context())
		if err != nil {
			return err
		}

		logIngestChannel <- db.LogLineWithIp{LogLine: logLine, Ip: ipv4}
	}
}

func (i *ImplementedServer) mustEmbedUnimplementedLoggAggregatorServer() {}

func newServer() rpc.LoggAggregatorServer {
	return &ImplementedServer{}
}

func loadEnv() Env {
	env := Env{}
	if os.Getenv("DOCKER") != "true" {
		err := dotenv.Load()

		if err != nil {
			log.Fatalf("Error loading .env file. %v", err)
		}
	}

	if err := envParser.Parse(&env); err != nil {
		log.Fatalf("Error parsing env: %+v\n", err)
	}

	return env
}

func startGrpcServer(env Env) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", env.GrpcServerPort))
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
	logServer := db.NewLogServer(mongoRepository)
	httpServer := http.NewServer(mongoRepository, http.HttpServerOptions{
		AllowedOrigins: env.HttpServerAllowedOrigins,
		ServerPort:     env.HttpServerPort,
	})

	slog.Info(fmt.Sprintf("Starting gRPC server on port %d, HTTP server on port %d\n", env.GrpcServerPort, env.HttpServerPort))

	go logServer.Run(context.Background(), logIngestChannel)
	go httpServer.Start()
	startGrpcServer(env)
}
