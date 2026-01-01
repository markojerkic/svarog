package testutils

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/lib/util"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/nats"
	"go.mongodb.org/mongo-driver/mongo"
)

// BaseSuite provides shared MongoDB and NATS infrastructure for all test suites.
// Embed this in your test suite to get access to both containers.
type BaseSuite struct {
	suite.Suite

	// MongoDB
	MongoContainer *MongoDBTestContainer
	MongoClient    *mongo.Client
	Database       *mongo.Database

	// NATS
	NatsContainer *nats.NATSContainer
	NatsAddr      string
	NatsConn      *natsconn.NatsConnection
	IssuerSeed    string

	// Services
	TokenService    *serverauth.TokenService
	AuthHandler     *serverauth.NatsAuthCalloutHandler
	ProjectsService projects.ProjectsService

	// WebSocket
	WatchHub      *websocket.WatchHub
	WsLogRenderer *websocket.WsLogLineRenderer

	// Config
	config BaseSuiteConfig
}

// BaseSuiteConfig holds configuration for the base test suite
type BaseSuiteConfig struct {
	// MongoDB
	DatabaseName string

	// NATS
	SystemUser     string
	SystemPassword string
	AppUser        string
	AppPassword    string
	JWTSecret      string
	NatsWsPort     string
	ConfigPath     string
}

// DefaultBaseSuiteConfig returns sensible defaults
func DefaultBaseSuiteConfig() BaseSuiteConfig {
	return BaseSuiteConfig{
		DatabaseName:   "svarog_test",
		SystemUser:     "system",
		SystemPassword: "password",
		AppUser:        "app",
		AppPassword:    "apppass",
		JWTSecret:      "test-jwt-secret",
		NatsWsPort:     "9222",
		ConfigPath:     "../../nats-server.conf",
	}
}

// WithConfig sets custom configuration for the suite
func (s *BaseSuite) WithConfig(config BaseSuiteConfig) {
	s.config = config
}

// SetupSuite starts MongoDB and optionally NATS containers
func (s *BaseSuite) SetupSuite() {
	ctx := context.Background()

	// Use default config if none set
	if s.config.DatabaseName == "" {
		s.config = DefaultBaseSuiteConfig()
	}

	util.SetupLogger()

	// Start MongoDB
	mongoContainer, err := NewMongoDBTestContainer(ctx, s.config.DatabaseName)
	if err != nil {
		s.T().Fatalf("failed to start MongoDB container: %v", err)
	}
	s.MongoContainer = mongoContainer
	s.MongoClient = mongoContainer.Client
	s.Database = mongoContainer.Database

	// Create projects service (always needed)
	projectsCollection := s.Database.Collection("projects")
	s.ProjectsService = projects.NewProjectsService(projectsCollection, s.MongoClient)

	// Start NATS
	if err := s.setupNats(ctx); err != nil {
		_ = mongoContainer.Terminate(ctx)
		s.T().Fatalf("failed to start NATS: %v", err)
	}
}

func (s *BaseSuite) setupNats(ctx context.Context) error {
	// Generate NATS issuer key pair
	issuerKp, err := nkeys.CreateAccount()
	if err != nil {
		return fmt.Errorf("failed to create issuer key pair: %w", err)
	}

	issuerSeed, err := issuerKp.Seed()
	if err != nil {
		return fmt.Errorf("failed to get issuer seed: %w", err)
	}
	s.IssuerSeed = string(issuerSeed)

	issuerPublicKey, err := issuerKp.PublicKey()
	if err != nil {
		return fmt.Errorf("failed to get issuer public key: %w", err)
	}

	// Read and substitute variables in nats-server.conf
	natsConfig, err := s.loadNatsConfig(issuerPublicKey)
	if err != nil {
		return fmt.Errorf("failed to load NATS config: %w", err)
	}

	// Start NATS container
	container, err := nats.Run(ctx, "nats:latest",
		nats.WithConfigFile(strings.NewReader(natsConfig)),
	)
	if err != nil {
		return fmt.Errorf("failed to start NATS container: %w", err)
	}
	s.NatsContainer = container

	natsAddr, err := container.ConnectionString(ctx)
	if err != nil {
		return fmt.Errorf("failed to get NATS connection string: %w", err)
	}
	s.NatsAddr = natsAddr

	// Create token service
	tokenService, err := serverauth.NewTokenService(s.config.JWTSecret)
	if err != nil {
		return fmt.Errorf("failed to create token service: %w", err)
	}
	s.TokenService = tokenService

	// Create NATS connection for auth callout (SYSTEM account)
	natsConn, err := natsconn.NewNatsConnection(natsconn.NatsConnectionConfig{
		NatsAddr:        natsAddr,
		User:            s.config.SystemUser,
		Password:        s.config.SystemPassword,
		EnableJetStream: true,
		JetStreamConfig: natsconn.JetStreamConfig{
			Name:     "LOGS",
			Subjects: []string{"logs.>"},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create NATS connection: %w", err)
	}
	s.NatsConn = natsConn

	// Create auth handler
	authHandler, err := serverauth.NewNatsAuthCalloutHandler(serverauth.NatsAuthConfig{
		IssuerSeed: s.IssuerSeed,
	}, natsConn.Conn, tokenService, s.ProjectsService)
	if err != nil {
		natsConn.Close()
		return fmt.Errorf("failed to create auth handler: %w", err)
	}
	s.AuthHandler = authHandler

	// Start the auth callout handler
	if err := authHandler.Run(); err != nil {
		natsConn.Close()
		return fmt.Errorf("failed to start auth callout handler: %w", err)
	}

	// Create WatchHub and WsLogLineRenderer
	s.WatchHub = websocket.NewWatchHub(natsConn.Conn)
	s.WsLogRenderer = websocket.NewWsLogLineRenderer(s.WatchHub)

	return nil
}

func (s *BaseSuite) loadNatsConfig(issuerPublicKey string) (string, error) {
	configBytes, err := os.ReadFile(s.config.ConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to read nats-server.conf: %w", err)
	}

	configStr := string(configBytes)

	replacer := strings.NewReplacer(
		"$NATS_WS_PORT", s.config.NatsWsPort,
		"$NATS_SYSTEM_USER", s.config.SystemUser,
		"$NATS_SYSTEM_PASSWORD", s.config.SystemPassword,
		"$NATS_APP_USER", s.config.AppUser,
		"$NATS_APP_PASSWORD", s.config.AppPassword,
		"$NATS_ISSUER_PUBLIC_KEY", issuerPublicKey,
	)

	return replacer.Replace(configStr), nil
}

// TearDownSuite cleans up all containers
func (s *BaseSuite) TearDownSuite() {
	ctx := context.Background()

	if s.NatsConn != nil {
		s.NatsConn.Close()
	}
	if s.NatsContainer != nil {
		_ = s.NatsContainer.Terminate(ctx)
	}
	if s.MongoContainer != nil {
		_ = s.MongoContainer.Terminate(ctx)
	}
}

// Collection is a helper to get a MongoDB collection
func (s *BaseSuite) Collection(name string) *mongo.Collection {
	return s.Database.Collection(name)
}
