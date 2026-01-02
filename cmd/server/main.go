package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"

	envParser "github.com/caarlos0/env/v11"
	dotenv "github.com/joho/godotenv"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http"
	"github.com/markojerkic/svarog/internal/server/ingest"
	"github.com/markojerkic/svarog/internal/server/types"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func loadEnv() types.ServerEnv {
	env := types.ServerEnv{}
	if os.Getenv("DOCKER") != "true" {
		err := dotenv.Load()

		if err != nil {
			log.Fatal("Error loading .env file", "error", err)
		}
	}

	if err := envParser.Parse(&env); err != nil {
		log.Fatal("Error parsing env", "error", err)
	}

	return env
}

func setupLogger() {
	util.SetupLogger()
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

type serverDependencies struct {
	httpServer    *http.HttpServer
	ingestService *ingest.IngestService
	natsConn      *natsconn.NatsConnection
	mongoClient   *mongo.Client
	cancel        context.CancelFunc
}

func gracefulShutdown(deps serverDependencies) {
	log.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := deps.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", "error", err)
	}

	deps.ingestService.Stop()

	deps.cancel()

	deps.natsConn.Close()

	if err := deps.mongoClient.Disconnect(shutdownCtx); err != nil {
		log.Error("MongoDB disconnect error", "error", err)
	}

	log.Info("Server stopped gracefully")
}

func main() {
	setupLogger()
	env := loadEnv()

	client, database, err := newMongoDB(env.MongoUrl)
	if err != nil {
		log.Fatal("Couldn't connect to Mongodb", "error", err)
	}

	userCollection := database.Collection("users")
	sessionCollection := database.Collection("sessions")
	filesCollectinon := database.Collection("files")
	projectsCollection := database.Collection("projects")

	// Create credential service for generating client credentials
	credentialService, err := serverauth.NewNatsCredentialService(env.NatsAccountSeed)
	if err != nil {
		log.Fatal("Failed to create credential service", "error", err)
	}
	// Log the account public key for verification/debugging
	log.Info("NATS credential service initialized", "accountPublicKey", credentialService.GetAccountPublicKey())

	// Use server user JWT credentials for NATS connections
	// Server user is in APP account with full permissions and JetStream access
	natsConn, err := natsconn.NewNatsConnection(natsconn.NatsConnectionConfig{
		NatsAddr:        env.NatsAddr,
		JWT:             env.NatsServerUserJWT,
		Seed:            env.NatsServerUserSeed,
		EnableJetStream: true,
		JetStreamConfig: natsconn.JetStreamConfig{
			Name:     "LOGS",
			Subjects: []string{"logs.>"},
		},
	})
	if err != nil {
		log.Fatal("Failed to connect to NATS", "error", err)
	}

	watchHub := websocket.NewWatchHub(natsConn.Conn)
	wsLoglineRenderer := websocket.NewWsLogLineRenderer(watchHub)

	sessionStore := auth.NewMongoSessionStore(sessionCollection, userCollection, []byte("secret"))
	logsService := db.NewLogService(database, wsLoglineRenderer)
	logServer := db.NewLogServer(logsService)

	authService := auth.NewMongoAuthService(userCollection, sessionCollection, client, sessionStore)
	filesService := files.NewFileService(filesCollectinon)
	projectsService := projects.NewProjectsService(projectsCollection, client)

	authService.CreateInitialAdminUser(context.Background())

	logIngestChannel := make(chan db.LogLineWithHost, 1000)
	ingestService := ingest.NewIngestService(logIngestChannel, natsConn)

	httpServer := http.NewServer(
		http.HttpServerOptions{
			AllowedOrigins:  env.HttpServerAllowedOrigins,
			ServerPort:      env.HttpServerPort,
			SessionStore:    sessionStore,
			LogService:      logsService,
			AuthService:     authService,
			FilesService:    filesService,
			ProjectsService: projectsService,
			WatchHub:        watchHub,
		})

	ctx, cancel := context.WithCancel(context.Background())

	// Signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go logServer.Run(ctx, logIngestChannel)
	go ingestService.Run(ctx)
	go func() {
		if err := httpServer.Start(); err != nil {
			log.Info("HTTP server stopped", "error", err)
		}
	}()

	<-quit
	gracefulShutdown(serverDependencies{
		httpServer:    httpServer,
		ingestService: ingestService,
		natsConn:      natsConn,
		mongoClient:   client,
		cancel:        cancel,
	})
}
