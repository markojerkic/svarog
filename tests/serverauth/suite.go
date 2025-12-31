package serverauth

import (
	"context"

	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NatsAuthSuite struct {
	suite.Suite

	natsContainer *testutils.NATSTestContainer
	tokenService  *serverauth.TokenService
	authHandler   *serverauth.NatsAuthCalloutHandler
}

func (s *NatsAuthSuite) SetupSuite() {
	t := s.T()
	ctx := context.Background()

	util.SetupLogger()

	config := testutils.DefaultNATSTestConfig()
	config.ConfigPath = "../../nats-server.conf"

	natsContainer, err := testutils.NewNATSTestContainer(ctx, config)
	require.NoError(t, err, "failed to start NATS container")

	s.natsContainer = natsContainer
	s.tokenService = natsContainer.TokenService
	s.authHandler = natsContainer.AuthHandler
}

func (s *NatsAuthSuite) TearDownSuite() {
	if s.natsContainer != nil {
		_ = s.natsContainer.Terminate(context.Background())
	}
}
