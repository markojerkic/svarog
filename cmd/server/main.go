package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go/jetstream"

	envParser "github.com/caarlos0/env/v11"
	dotenv "github.com/joho/godotenv"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http"
	"github.com/markojerkic/svarog/internal/server/ingest"
	"github.com/markojerkic/svarog/internal/server/types"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func loadEnv() types.ServerEnv {
	env := types.ServerEnv{}
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
	projectsCollection := database.Collection("projects")

	sessionStore := auth.NewMongoSessionStore(sessionCollection, userCollection, []byte("secret"))
	logsService := db.NewLogService(database)
	logServer := db.NewLogServer(logsService)

	authService := auth.NewMongoAuthService(userCollection, sessionCollection, client, sessionStore)
	filesService := files.NewFileService(filesCollectinon)
	certificateService := serverauth.NewCertificateService(filesService, client, strings.Split(env.ServerDnsName, ","))
	projectsService := projects.NewProjectsService(projectsCollection, client)

	authService.CreateInitialAdminUser(context.Background())

	tokenService, err := serverauth.NewTokenService(env.NatsJwtSecret)
	if err != nil {
		log.Fatalf("Failed to create token service: %v", err)
	}

	natsSystemConn, err := serverauth.NewNatsConnection(serverauth.NatsConnectionConfig{
		NatsAddr: env.NatsAddr,
		User:     env.NatsSystemUser,
		Password: env.NatsSystemPassword,
	})
	if err != nil {
		log.Fatalf("Failed to connect to NATS SYSTEM account: %v", err)
	}

	natsAppConn, err := serverauth.NewNatsConnection(serverauth.NatsConnectionConfig{
		NatsAddr:        env.NatsAddr,
		User:            env.NatsAppUser,
		Password:        env.NatsAppPassword,
		EnableJetStream: true,
	})
	if err != nil {
		log.Fatalf("Failed to connect to NATS APP account: %v", err)
	}

	logIngestChannel := make(chan db.LogLineWithHost, 1000)
	ingestService := ingest.NewIngestService(logIngestChannel, natsAppConn)

	natsAuthConfig := serverauth.NatsAuthConfig{
		IssuerSeed: env.NatsIssuerSeed,
	}
	natsAuthService, err := serverauth.NewNatsAuthCalloutHandler(
		natsAuthConfig,
		natsSystemConn.Conn,
		tokenService)
	if err != nil {
		log.Fatalf("Failed to create NATS auth handler: %v", err)
	}

	httpServer := http.NewServer(
		http.HttpServerOptions{
			AllowedOrigins:     env.HttpServerAllowedOrigins,
			ServerPort:         env.HttpServerPort,
			SessionStore:       sessionStore,
			LogService:         logsService,
			AuthService:        authService,
			CertificateService: certificateService,
			FilesService:       filesService,
			ProjectsService:    projectsService,
		})

	go natsAuthService.Run()
	go logServer.Run(context.Background(), logIngestChannel)
	go ingestService.Run(context.Background())
	httpServer.Start()
}
