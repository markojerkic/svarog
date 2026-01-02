package serverauth

import (
	"context"

	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/tests/testutils"
	"go.mongodb.org/mongo-driver/bson"
)

type NatsAuthSuite struct {
	testutils.BaseSuite

	credentialService *serverauth.NatsCredentialService
}

func (s *NatsAuthSuite) SetupSuite() {
	s.BaseSuite.SetupSuite()

	s.credentialService = s.CredentialService
}

func (s *NatsAuthSuite) TearDownTest() {
	// Clean up projects between tests
	_, _ = s.Collection("projects").DeleteMany(context.Background(), bson.M{})
}

func (s *NatsAuthSuite) TearDownSuite() {
	s.BaseSuite.TearDownSuite()
}
