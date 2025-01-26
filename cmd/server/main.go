package main

import (
	"context"
	"errors"

	"github.com/charmbracelet/log"

	envParser "github.com/caarlos0/env/v11"
	dotenv "github.com/joho/godotenv"
	"github.com/markojerkic/svarog/internal/grpcserver"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http"
	"github.com/markojerkic/svarog/internal/server/types"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func loadEnv() types.ServerEnv {
	env := types.ServerEnv{}
	err := dotenv.Load()
	log.Info("Loading .env file")

	if err != nil {
		log.Warn("Error loading .env file. Falling back to OS env parsing - ", err)
	} else {
		return env
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
	logsRepository := db.NewLogRepository(database)
	logServer := db.NewLogServer(logsRepository)

	authService := auth.NewMongoAuthService(userCollection, sessionCollection, client, sessionStore)
	filesService := files.NewFileService(filesCollectinon)
	certificateService := serverauth.NewCertificateService(filesService, client)
	projectsService := projects.NewProjectsService(projectsCollection, client)

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
			ProjectsService:    projectsService,
		})

	logIngestChannel := make(chan db.LogLineWithIp, 1000)
	grpcServer := grpcserver.NewGrpcServer(certificateService, projectsService, env, logIngestChannel)

	go logServer.Run(context.Background(), logIngestChannel)
	go httpServer.Start()
	grpcServer.Start()
}
