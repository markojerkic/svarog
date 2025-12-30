package natsauth

import (
	"context"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/nats"
)

const (
	systemUser     = "system"
	systemPassword = "password"
	jwtSecret      = "test-jwt-secret"
	natsWsPort     = "9222"
)

type NatsAuthSuite struct {
	suite.Suite

	container   *nats.NATSContainer
	natsAddr    string
	authHandler *serverauth.NatsAuthCalloutHandler
}

func (s *NatsAuthSuite) SetupSuite() {
	t := s.T()
	ctx := context.Background()

	log.SetLevel(log.DebugLevel)

	// Generate NATS issuer key pair
	issuerKp, err := nkeys.CreateAccount()
	require.NoError(t, err, "failed to create issuer key pair")

	issuerSeed, err := issuerKp.Seed()
	require.NoError(t, err, "failed to get issuer seed")

	issuerPublicKey, err := issuerKp.PublicKey()
	require.NoError(t, err, "failed to get issuer public key")

	// Read and substitute variables in nats-server.conf
	natsConfig := s.loadNatsConfig(issuerPublicKey)

	// Start NATS container with config (2.10+ required for auth_callout)
	container, err := nats.Run(ctx, "nats:latest",
		nats.WithConfigFile(strings.NewReader(natsConfig)),
	)
	require.NoError(t, err, "failed to start NATS container")
	s.container = container

	s.natsAddr, err = container.ConnectionString(ctx)
	require.NoError(t, err, "failed to get NATS connection string")

	// Create auth handler with explicit config
	s.authHandler, err = serverauth.NewNatsAuthCalloutHandler(serverauth.NatsAuthConfig{
		IssuerSeed:     string(issuerSeed),
		JwtSecret:      jwtSecret,
		SystemUser:     systemUser,
		SystemPassword: systemPassword,
		NatsAddr:       s.natsAddr,
	})
	require.NoError(t, err, "failed to create auth handler")

	// Start the auth callout handler
	err = s.authHandler.Run()
	require.NoError(t, err, "failed to start auth callout handler")
}

func (s *NatsAuthSuite) TearDownSuite() {
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}
}

func (s *NatsAuthSuite) loadNatsConfig(issuerPublicKey string) string {
	t := s.T()

	configBytes, err := os.ReadFile("../../../nats-server.conf")
	require.NoError(t, err, "failed to read nats-server.conf")

	config := string(configBytes)

	// Substitute environment variables
	replacer := strings.NewReplacer(
		"$NATS_WS_PORT", natsWsPort,
		"$NATS_SYSTEM_USER", systemUser,
		"$NATS_SYSTEM_PASSWORD", systemPassword,
		"$NATS_ISSUER_PUBLIC_KEY", issuerPublicKey,
	)

	return replacer.Replace(config)
}
