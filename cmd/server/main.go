package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	envParser "github.com/caarlos0/env/v11"
	"github.com/charmbracelet/log"
	"github.com/jackc/pgx/v5/pgxpool"
	dotenv "github.com/joho/godotenv"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/lib/svaroglog"
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
	return env
}

type serverDependencies struct {
	httpServer    *http.HttpServer
	ingestService *ingest.Service
	cancel        context.CancelFunc
	background    *sync.WaitGroup
}

func gracefulShutdown(deps serverDependencies) {
	slog.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	httpStopped := make(chan struct{})
	go func() {
		defer close(httpStopped)
		if err := deps.httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}
	}()
	if err := deps.ingestService.Shutdown(shutdownCtx); err != nil {
		slog.Error("Ingest shutdown error", "error", err)
	}
	<-httpStopped
	deps.cancel()
	// Canceled jobs must exit before closing their borrowed DB/NATS resources.
	deps.background.Wait()
}

func main() {
	env := loadEnv()
	level, err := util.ParseLogLevel(env.LogLevel)
	if err != nil {
		log.Fatal("Invalid log level", "error", err)
	}
	util.SetupLogger(level)
	var closeLogging func() error
	if env.LogConnectionString != "" {
		closeLogging, err = svaroglog.Setup(context.Background(),
			svaroglog.WithConnectionString(env.LogConnectionString),
			svaroglog.WithConsole(log.Default()),
			svaroglog.WithLevel(slog.Level(level)),
			svaroglog.WithDropWhenFull(),
		)
		if err != nil {
			log.Error("Cannot forward server logs; using console only", "error", err)
		}
	}

	err = runServer(env)
	if err != nil {
		slog.Error("Server failed", "error", err)
	} else {
		slog.Info("Server stopped gracefully")
	}
	if closeLogging != nil {
		if drainErr := closeLogging(); drainErr != nil {
			log.Error("Could not flush server logs to Svarog", "error", drainErr)
		}
	}
	if err != nil {
		os.Exit(1)
	}
}

func runServer(env types.ServerEnv) error {
	if err := migrations.Up(context.Background(), env.PostgresURL); err != nil {
		return fmt.Errorf("apply PostgreSQL migrations: %w", err)
	}
	slog.Info("PostgreSQL migrations applied")

	postgresCtx, postgresCancel := context.WithTimeout(context.Background(), 10*time.Second)
	postgresPool, err := pgxpool.New(postgresCtx, env.PostgresURL)
	if err == nil {
		defer postgresPool.Close()
		err = postgresPool.Ping(postgresCtx)
	}
	postgresCancel()
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	projectsService := projects.NewPostgresProjectsService(postgresPool)
	archiveStore, err := newArchiveStore(env)
	if err != nil {
		return fmt.Errorf("configure archive storage: %w", err)
	}
	projectsService.SetArchiveStore(archiveStore)
	archiver := archive.NewArchiveService(postgresPool, archiveStore)

	natsCredService, err := serverauth.NewNatsCredentialService(env.NatsAccountSeed, env.NatsPublicAddr, projectsService)
	if err != nil {
		return fmt.Errorf("create credential service: %w", err)
	}

	slog.Debug("Starting NATS connection", "addr", env.NatsAddr, "publicAddr", env.NatsPublicAddr)
	natsConn, err := natsconn.NewNatsConnection(natsconn.NatsConnectionConfig{
		NatsAddr:        env.NatsAddr,
		JWT:             env.NatsServerUserJWT,
		Seed:            env.NatsServerUserSeed,
		EnableJetStream: true,
	})
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	defer natsConn.Close()

	liveConn, err := natsconn.NewNatsConnection(natsconn.NatsConnectionConfig{
		NatsAddr:               env.NatsAddr,
		JWT:                    env.NatsServerUserJWT,
		Seed:                   env.NatsServerUserSeed,
		DisableReconnectBuffer: true,
	})
	if err != nil {
		return fmt.Errorf("connect live logs to NATS: %w", err)
	}
	defer liveConn.Close()
	liveLogs := live.New(liveConn.Conn)
	defer liveLogs.Close()

	sessionStore := auth.NewPostgresSessionStore(postgresPool, []byte(env.SessionSecret))
	logsService := db.NewLogService(postgresPool, liveLogs)

	authService := auth.NewPostgresAuthService(postgresPool, sessionStore)

	if err := authService.CreateInitialAdminUser(context.Background()); err != nil {
		return fmt.Errorf("initialize PostgreSQL admin user: %w", err)
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	ingestService, err := ingest.Open(startupCtx, ingest.Options{
		Connection: natsConn, Pool: postgresPool, Writer: logsService,
		StreamMaxBytes: env.IngestStreamMaxBytes, AckWait: env.IngestAckWait,
		AllowRetentionMigration: env.IngestAllowRetentionMigration,
	})
	startupCancel()
	if err != nil {
		return fmt.Errorf("prepare durable log ingestion: %w", err)
	}

	httpServer := http.NewServer(
		http.HttpServerOptions{
			ServerPort:              env.HttpServerPort,
			SessionStore:            sessionStore,
			LogService:              logsService,
			AuthService:             authService,
			ProjectsService:         projectsService,
			Archiver:                archiver,
			ArchiveSchedulerEnabled: env.ArchiveSchedulerEnabled && archiveStore != nil,
			NatsCredentialService:   natsCredService,
			LiveLogs:                liveLogs,
		})

	ctx, cancel := context.WithCancel(context.Background())

	// Signal handling for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	ingestDone := make(chan error, 1)
	go func() { ingestDone <- ingestService.Run(ctx) }()
	var background sync.WaitGroup
	if archiveStore == nil {
		slog.Info("Archive storage not configured; archive scheduler not started")
	} else if !env.ArchiveSchedulerEnabled {
		slog.Info("Archive scheduler disabled")
	} else {
		background.Go(func() { archive.RunScheduler(ctx, archiver) })
	}
	httpDone := make(chan error, 1)
	go func() { httpDone <- httpServer.Start() }()

	var stopErr error
	select {
	case <-quit:
	case err := <-ingestDone:
		stopErr = fmt.Errorf("ingest stopped unexpectedly: %w", err)
	case err := <-httpDone:
		stopErr = fmt.Errorf("HTTP server stopped unexpectedly: %w", err)
	}
	gracefulShutdown(serverDependencies{
		httpServer:    httpServer,
		ingestService: ingestService,
		cancel:        cancel,
		background:    &background,
	})
	return stopErr
}
