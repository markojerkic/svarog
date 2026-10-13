package serverauth

import (
	"context"

	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

type NatsAuthSuite struct {
	testutils.BaseSuite

	credentialService *serverauth.NatsCredentialService
}

func (s *NatsAuthSuite) SetupSuite() {
	s.BaseSuite.SetupSuite()
}

func (s *NatsAuthSuite) TearDownTest() {
	_, err := s.PostgresPool.Exec(context.Background(), "TRUNCATE projects CASCADE")
	require.NoError(s.T(), err, "clear PostgreSQL projects and clients between credential tests")
}

func (s *NatsAuthSuite) TearDownSuite() {
	s.BaseSuite.TearDownSuite()
}
