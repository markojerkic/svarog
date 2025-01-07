package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/charmbracelet/log"

	envParser "github.com/caarlos0/env/v11"
	dotenv "github.com/joho/godotenv"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	log.Debug("Received batch log", "size", int64(len(batchLogs.Logs)), "ip", ipv4)

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
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)
}

func newMongoDB(connectionUrl string) (*mongo.Client, *mongo.Database, error) {
	clientOptions := options.Client().ApplyURI(connectionUrl)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, nil, errors.Join(errors.New("Error connecting to MongoDb"), err)
	}

	database := client.Database("logs")

	return client, database, nil
}

func main() {
	setupLogger()
	env := loadEnv()

	client, database, err := newMongoDB(env.MongoUrl)
	if err != nil {
		log.Fatalf("Couldn't connect to Mongodb: %+v", err)
	}

	userCollection := database.Collection("users")
	sessionCollection := database.Collection("sessions")
	filesCollectinon := database.Collection("files")

	sessionStore := auth.NewMongoSessionStore(sessionCollection, userCollection, []byte("secret"))
	logsRepository := db.NewLogRepository(database)
	logServer := db.NewLogServer(logsRepository)

	authService := auth.NewMongoAuthService(userCollection, sessionCollection, client, sessionStore)
	filesService := files.NewFileService(filesCollectinon)
	certificateService := serverauth.NewCertificateService(filesService, client)

	authService.CreateInitialAdminUser(context.Background())

	httpServer := http.NewServer(
		http.HttpServerOptions{
			AllowedOrigins:     env.HttpServerAllowedOrigins,
			ServerPort:         env.HttpServerPort,
			SessionStore:       sessionStore,
			LogRepository:      logsRepository,
			AuthService:        authService,
			CertificateService: certificateService,
			FilesService:       filesService,
		})

	log.Info(fmt.Sprintf("Starting gRPC server on port %d, HTTP server on port %d", env.GrpcServerPort, env.HttpServerPort))

	go logServer.Run(context.Background(), logIngestChannel)
	go httpServer.Start()
	startGrpcServer(env)
}
