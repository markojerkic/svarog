package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"

	envParser "github.com/caarlos0/env/v11"
	"github.com/jackc/pgx/v5/pgxpool"
	dotenv "github.com/joho/godotenv"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http"
	"github.com/markojerkic/svarog/internal/server/ingest"
	"github.com/markojerkic/svarog/internal/server/types"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
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

type serverDependencies struct {
	httpServer    *http.HttpServer
	ingestService *ingest.IngestService
	natsConn      *natsconn.NatsConnection
	postgresPool  *pgxpool.Pool
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
	deps.postgresPool.Close()

	log.Info("Server stopped gracefully")
}

func main() {
	setupLogger()
	env := loadEnv()
	postgresCtx, postgresCancel := context.WithTimeout(context.Background(), 10*time.Second)
	postgresPool, err := pgxpool.New(postgresCtx, env.PostgresURL)
	if err == nil {
		err = postgresPool.Ping(postgresCtx)
	}
	postgresCancel()
	if err != nil {
		log.Fatal("Couldn't connect to PostgreSQL", "error", err)
	}

	projectsService := projects.NewPostgresProjectsService(postgresPool)
	archiver := archive.NewArchiveService(postgresPool, archive.NewDiskStore(env.ArchiveDir))

	natsCredService, err := serverauth.NewNatsCredentialService(env.NatsAccountSeed, env.NatsPublicAddr, projectsService)
	if err != nil {
		log.Fatal("Failed to create credential service", "error", err)
	}

	slog.Debug("Starting NATS connection", "addr", env.NatsAddr, "publicAddr", env.NatsPublicAddr)
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

	sessionStore := auth.NewPostgresSessionStore(postgresPool, []byte(env.SessionSecret))
	logsService := db.NewLogService(postgresPool, wsLoglineRenderer)
	logServer := db.NewLogServer(logsService)

	authService := auth.NewPostgresAuthService(postgresPool, sessionStore)

	if err := authService.CreateInitialAdminUser(context.Background()); err != nil {
		log.Fatal("Couldn't initialize PostgreSQL admin user; apply database migrations first", "error", err)
	}

	logIngestChannel := make(chan db.LogLineWithHost, 1000)
	ingestService := ingest.NewIngestService(logIngestChannel, natsConn, postgresPool)

	httpServer := http.NewServer(
		http.HttpServerOptions{
			ServerPort:            env.HttpServerPort,
			SessionStore:          sessionStore,
			LogService:            logsService,
			AuthService:           authService,
			ProjectsService:       projectsService,
			Archiver:              archiver,
			NatsCredentialService: natsCredService,
			WatchHub:              watchHub,
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
		postgresPool:  postgresPool,
		cancel:        cancel,
	})
}
