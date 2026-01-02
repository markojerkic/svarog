package testutils

import (
	"context"
	"fmt"
	"os"
	"strings"

	envParser "github.com/caarlos0/env/v11"
	dotenv "github.com/joho/godotenv"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/types"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
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

	// Services
	NatsCredsService *serverauth.NatsCredentialService
	ProjectsService  projects.ProjectsService

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
	ConfigPath string
	NatsJwt    string
	NatsSeed   string
}

// DefaultBaseSuiteConfig returns sensible defaults
func DefaultBaseSuiteConfig() BaseSuiteConfig {
	env := types.ServerEnv{}
	if err := dotenv.Load("../../.env"); err != nil {
		panic(err)
	}
	if err := envParser.Parse(&env); err != nil {
		panic(err)
	}

	return BaseSuiteConfig{
		DatabaseName: "svarog_test",
		ConfigPath:   "../../nats-server.conf",
		NatsJwt:      env.NatsServerUserJWT,
		NatsSeed:     env.NatsServerUserSeed,
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
	natsConfig, err := os.ReadFile(s.config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load NATS config: %w", err)
	}

	// Start NATS container
	container, err := nats.Run(ctx, "nats:latest",
		nats.WithConfigFile(strings.NewReader(string(natsConfig))),
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
	tokenService, err := serverauth.NewNatsCredentialService(s.config.NatsJwt)
	if err != nil {
		return fmt.Errorf("failed to create token service: %w", err)
	}
	s.NatsCredsService = tokenService

	// Create NATS connection for auth callout (SYSTEM account)
	natsConn, err := natsconn.NewNatsConnection(natsconn.NatsConnectionConfig{
		NatsAddr:        natsAddr,
		JWT:             s.config.NatsJwt,
		Seed:            s.config.NatsSeed,
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

	// Create WatchHub and WsLogLineRenderer
	s.WatchHub = websocket.NewWatchHub(natsConn.Conn)
	s.WsLogRenderer = websocket.NewWsLogLineRenderer(s.WatchHub)

	return nil
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
