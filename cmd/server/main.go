package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
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
	"github.com/markojerkic/svarog/internal/persistence/postgres/migrations"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http"
	"github.com/markojerkic/svarog/internal/server/ingest"
	"github.com/markojerkic/svarog/internal/server/live"
	"github.com/markojerkic/svarog/internal/server/types"
)

func loadEnv() types.ServerEnv {
	env := types.ServerEnv{}
	if os.Getenv("DOCKER") != "true" {
		// Local values override .env defaults, but never the process environment.
		if err := dotenv.Load(".env.local"); err != nil && !os.IsNotExist(err) {
			log.Fatal("Error loading .env.local file", "error", err)
		}
		err := dotenv.Load()

		if err != nil {
			log.Fatal("Error loading .env file", "error", err)
		}
	}

	if err := envParser.Parse(&env); err != nil {
		log.Fatal("Error parsing env", "error", err)
	}
	if env.ArchiveScheduleInterval < 0 {
		log.Fatal("ARCHIVE_SCHEDULE_INTERVAL must not be negative", "value", env.ArchiveScheduleInterval)
	}

	return env
}

type serverDependencies struct {
	httpServer    *http.HttpServer
	ingestService *ingest.Service
	natsConn      *natsconn.NatsConnection
	liveLogs      *live.Logs
	liveConn      *natsconn.NatsConnection
	postgresPool  *pgxpool.Pool
	cancel        context.CancelFunc
	background    *sync.WaitGroup
}

func gracefulShutdown(deps serverDependencies) {
	log.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	httpStopped := make(chan struct{})
	go func() {
		defer close(httpStopped)
		if err := deps.httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error("HTTP server shutdown error", "error", err)
		}
	}()
	if err := deps.ingestService.Shutdown(shutdownCtx); err != nil {
		log.Error("Ingest shutdown error", "error", err)
	}
	<-httpStopped
	deps.cancel()
	// Canceled jobs must exit before closing their borrowed DB/NATS resources.
	deps.background.Wait()

	deps.liveLogs.Close()
	deps.liveConn.Close()
	deps.natsConn.Close()
	deps.postgresPool.Close()

	log.Info("Server stopped gracefully")
}

func main() {
	env := loadEnv()
	level, err := util.ParseLogLevel(env.LogLevel)
	if err != nil {
		log.Fatal("Invalid log level", "error", err)
	}
	util.SetupLogger(level)

	if err := migrations.Up(context.Background(), env.PostgresURL); err != nil {
		log.Fatal("Couldn't apply PostgreSQL migrations", "error", err)
	}
	slog.Info("PostgreSQL migrations applied")

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
	archiveStore, err := newArchiveStore(env)
	if err != nil {
		log.Fatal("Failed to configure archive storage", "error", err)
	}
	projectsService.SetArchiveStore(archiveStore)
	archiver := archive.NewArchiveService(postgresPool, archiveStore)

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
	})
	if err != nil {
		log.Fatal("Failed to connect to NATS", "error", err)
	}

	liveConn, err := natsconn.NewNatsConnection(natsconn.NatsConnectionConfig{
		NatsAddr:               env.NatsAddr,
		JWT:                    env.NatsServerUserJWT,
		Seed:                   env.NatsServerUserSeed,
		DisableReconnectBuffer: true,
	})
	if err != nil {
		log.Fatal("Failed to connect live logs to NATS", "error", err)
	}
	liveLogs := live.New(liveConn.Conn)

	sessionStore := auth.NewPostgresSessionStore(postgresPool, []byte(env.SessionSecret))
	logsService := db.NewLogService(postgresPool, liveLogs)

	authService := auth.NewPostgresAuthService(postgresPool, sessionStore)

	if err := authService.CreateInitialAdminUser(context.Background()); err != nil {
		log.Fatal("Couldn't initialize PostgreSQL admin user", "error", err)
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	ingestService, err := ingest.Open(startupCtx, ingest.Options{
		Connection: natsConn, Pool: postgresPool, Writer: logsService,
		StreamMaxBytes: env.IngestStreamMaxBytes, AckWait: env.IngestAckWait,
		AllowRetentionMigration: env.IngestAllowRetentionMigration,
	})
	startupCancel()
	if err != nil {
		log.Fatal("Failed to prepare durable log ingestion", "error", err)
	}

	httpServer := http.NewServer(
		http.HttpServerOptions{
			ServerPort:            env.HttpServerPort,
			SessionStore:          sessionStore,
			LogService:            logsService,
			AuthService:           authService,
			ProjectsService:       projectsService,
			Archiver:              archiver,
			NatsCredentialService: natsCredService,
			LiveLogs:              liveLogs,
		})

	ctx, cancel := context.WithCancel(context.Background())

	// Signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	ingestDone := make(chan error, 1)
	go func() { ingestDone <- ingestService.Run(ctx) }()
	var background sync.WaitGroup
	if archiveStore != nil {
		background.Go(func() { archive.RunScheduler(ctx, archiver, env.ArchiveScheduleInterval) })
	} else {
		slog.Info("Archive storage not configured; archive scheduler not started")
	}
	httpDone := make(chan error, 1)
	go func() { httpDone <- httpServer.Start() }()

	failed := false
	select {
	case <-quit:
	case err := <-ingestDone:
		failed = true
		log.Error("Ingest stopped unexpectedly", "error", err)
	case err := <-httpDone:
		failed = true
		log.Error("HTTP server stopped unexpectedly", "error", err)
	}
	gracefulShutdown(serverDependencies{
		httpServer:    httpServer,
		ingestService: ingestService,
		natsConn:      natsConn,
		liveLogs:      liveLogs,
		liveConn:      liveConn,
		postgresPool:  postgresPool,
		cancel:        cancel,
		background:    &background,
	})
	if failed {
		os.Exit(1)
	}
}
