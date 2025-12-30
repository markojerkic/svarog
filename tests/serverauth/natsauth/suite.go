package natsauth

import (
	"context"
	"os"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/nats"
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

	// Set environment variables for NatsAuthCalloutHandler
	os.Setenv("NATS_ISSUER_SEED", string(issuerSeed))
	os.Setenv("NATS_JWT_SECRET", "test-jwt-secret")
	os.Setenv("NATS_SYSTEM_USER", "system")
	os.Setenv("NATS_SYSTEM_PASSWORD", "password")
	os.Setenv("NATS_ISSUER_PUBLIC_KEY", issuerPublicKey)

	// Start NATS container
	container, err := nats.Run(ctx, "nats:2.9")
	require.NoError(t, err, "failed to start NATS container")
	s.container = container

	s.natsAddr, err = container.ConnectionString(ctx)
	require.NoError(t, err, "failed to get NATS connection string")

	os.Setenv("NATS_ADDR", s.natsAddr)

	s.authHandler = serverauth.NewNatsAuthCalloutHandler()
}

func (s *NatsAuthSuite) TearDownSuite() {
	if s.container != nil {
		_ = s.container.Terminate(context.Background())
	}

	// Clean up environment variables
	os.Unsetenv("NATS_ISSUER_SEED")
	os.Unsetenv("NATS_JWT_SECRET")
	os.Unsetenv("NATS_SYSTEM_USER")
	os.Unsetenv("NATS_SYSTEM_PASSWORD")
	os.Unsetenv("NATS_ISSUER_PUBLIC_KEY")
	os.Unsetenv("NATS_ADDR")
}
