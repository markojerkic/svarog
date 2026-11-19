package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/lib/svarogclient"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/persistence/postgres/migrations"
	"github.com/markojerkic/svarog/internal/server/http"
	"github.com/markojerkic/svarog/internal/server/ingest"
	"github.com/markojerkic/svarog/internal/server/live"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/markojerkic/svarog/svaroglog"
)

type App struct {
	config Config
	broker natsconn.Broker
}

type Option func(*App)

func WithBroker(broker natsconn.Broker) Option {
	return func(app *App) { app.broker = broker }
}

func New(cfg Config, opts ...Option) *App {
	app := &App{config: cfg}
	for _, opt := range opts {
		opt(app)
	}
	return app
}

func selfLogQueueCapacity(raw string) int {
	const defaultCapacity = svarogclient.DefaultDropQueueCapacity
	if raw == "" {
		return defaultCapacity
	}
	capacity, err := strconv.Atoi(raw)
	if err != nil || capacity < 1 || capacity > svarogclient.MaxDropQueueCapacity {
		log.Warn("Invalid SVAROG_LOG_QUEUE_CAPACITY; using default", "min", 1, "max", svarogclient.MaxDropQueueCapacity, "default", defaultCapacity)
		return defaultCapacity
	}
	return capacity
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

func Main(cfg Config, opts ...Option) int {
	level, err := util.ParseLogLevel(cfg.LogLevel)
	if err != nil {
		log.Error("Invalid log level", "error", err)
		return 1
	}
	util.SetupLogger(level)

	if err := New(cfg, opts...).Run(); err != nil {
		return 1
	}
	return 0
}

// Run serves until SIGINT/SIGTERM or a fatal error. It logs its own outcome so
// that the final record is still forwarded when self-logging is enabled.
func (a *App) Run() (runErr error) {
	reportOutcome := sync.OnceFunc(func() {
		if runErr != nil {
			slog.Error("Server failed", "error", runErr)
		} else {
			slog.Info("Server stopped gracefully")
		}
	})
	defer reportOutcome()
	if a.broker == nil {
		return errors.New("a NATS broker is required")
	}
	env := a.config

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

	// Validate the remaining configuration before the broker opens its store.
	archiveStore, err := newArchiveStore(env)
	if err != nil {
		return fmt.Errorf("configure archive storage: %w", err)
	}
	projectsService := projects.NewPostgresProjectsService(postgresPool, archiveStore)
	archiver := archive.NewArchiveService(postgresPool, archiveStore)

	natsCredService, err := serverauth.NewNatsCredentialService(env.NatsAccountSeed, env.NatsPublicAddr, projectsService)
	if err != nil {
		return fmt.Errorf("create credential service: %w", err)
	}

	// From here on a signal stops startup and the broker shuts down cleanly.
	signals, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	if err := a.broker.Start(signals); err != nil {
		return fmt.Errorf("start NATS broker: %w", err)
	}
	defer func() {
		// Wait for the broker to flush its store; the supervisor's stop grace
		// period bounds this wait.
		if err := a.broker.Shutdown(context.Background()); err != nil {
			err = fmt.Errorf("shut down NATS broker: %w", err)
			slog.Error("NATS broker shutdown failed", "error", err)
			runErr = errors.Join(runErr, err)
		}
	}()

	if env.LogConnectionString != "" {
		console := log.Default()
		closeLogging, err := svaroglog.Setup(context.Background(),
			svaroglog.WithConnectionString(env.LogConnectionString),
			svaroglog.WithConsole(console),
			svaroglog.WithLevel(slog.Level(console.GetLevel())),
			svaroglog.WithDropQueueCapacity(selfLogQueueCapacity(env.LogQueueCapacity)),
		)
		if err != nil {
			log.Error("Cannot forward server logs; using console only", "error", err)
		} else {
			defer func() {
				reportOutcome()
				if err := closeLogging(); err != nil {
					log.Error("Could not flush server logs to Svarog", "error", err)
				}
			}()
		}
	}

	natsConn, err := a.broker.Connect(natsconn.ConnectOptions{
		Name:            "svarog-ingest",
		EnableJetStream: true,
	})
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	defer natsConn.Close()

	liveConn, err := a.broker.Connect(natsconn.ConnectOptions{
		Name:                   "svarog-live",
		DisableReconnectBuffer: true,
	})
	if err != nil {
		return fmt.Errorf("connect live logs to NATS: %w", err)
	}
	defer liveConn.Close()
	liveLogs := live.New(liveConn.Conn)
	defer liveLogs.Close()

	sessionStore := auth.NewPostgresSessionStore(postgresPool, []byte(env.SessionSecret))
	logStore := logstore.New(postgresPool)

	authService := auth.NewPostgresAuthService(postgresPool, sessionStore)

	if err := authService.CreateInitialAdminUser(context.Background()); err != nil {
		return fmt.Errorf("initialize PostgreSQL admin user: %w", err)
	}

	startupCtx, startupCancel := context.WithTimeout(signals, 30*time.Second)
	ingestService, err := ingest.Open(startupCtx, ingest.Options{
		Connection: natsConn, Pool: postgresPool, Writer: logStore, Live: liveLogs,
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
			LogReader:               logStore,
			AuthService:             authService,
			ProjectsService:         projectsService,
			Archiver:                archiver,
			ArchiveSchedulerEnabled: env.ArchiveSchedulerEnabled && archiveStore != nil,
			NatsCredentialService:   natsCredService,
			LiveLogs:                liveLogs,
		})

	// Background work stops in gracefulShutdown's order, not directly on a signal.
	ctx, cancel := context.WithCancel(context.Background())

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
	case <-signals.Done():
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
