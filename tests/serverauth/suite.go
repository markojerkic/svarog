package serverauth

import (
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/tests/testutils"
)

type NatsAuthSuite struct {
	testutils.BaseSuite

	tokenService *serverauth.TokenService
	authHandler  *serverauth.NatsAuthCalloutHandler
}

func (s *NatsAuthSuite) SetupSuite() {
	config := testutils.DefaultBaseSuiteConfig()
	config.EnableNats = true // NATS auth tests need NATS
	s.WithConfig(config)

	s.BaseSuite.SetupSuite()

	s.tokenService = s.TokenService
	s.authHandler = s.AuthHandler
}

func (s *NatsAuthSuite) TearDownSuite() {
	s.BaseSuite.TearDownSuite()
}
