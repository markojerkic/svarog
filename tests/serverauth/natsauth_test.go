package serverauth

import (
	"context"
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestNatsAuthSuite(t *testing.T) {
	suite.Run(t, new(NatsAuthSuite))
}

func (s *NatsAuthSuite) TestGenerateCredentials() {
	t := s.T()

	creds, err := s.NatsCredsService.GenerateUserCreds(
		context.Background(),
		serverauth.CredentialGenerationRequest{
			ProjectID: "client-user-123",
			ClientID:  "topic",
		},
	)
	require.NoError(t, err)
	assert.NotEmpty(t, creds)
}

func (s *NatsAuthSuite) TestGenerateCredentialsWithExpiry() {
	t := s.T()
	expiry := 24 * time.Hour

	creds, err := s.NatsCredsService.GenerateUserCreds(
		context.Background(),
		serverauth.CredentialGenerationRequest{
			ProjectID: "client-user-123",
			ClientID:  "topic",
			Expiry:    time.Now().Add(expiry),
		},
	)
	require.NoError(t, err)
	assert.NotEmpty(t, creds)
}

func (s *NatsAuthSuite) TestGenerateCredsFile() {
	t := s.T()

	creds, err := s.NatsCredsService.GenerateUserCreds(
		context.Background(),
		serverauth.CredentialGenerationRequest{
			ProjectID: "client-user-123",
			ClientID:  "topic",
		},
	)
	assert.NoError(t, err)

	assert.Contains(t, creds, "-----BEGIN NATS USER JWT-----")
	assert.Contains(t, creds, "------END NATS USER JWT------")
	assert.Contains(t, creds, "-----BEGIN USER NKEY SEED-----")
	assert.Contains(t, creds, "------END USER NKEY SEED------")
}

func (s *NatsAuthSuite) TestParseCredsFile() {
	t := s.T()

	creds, err := s.NatsCredsService.GenerateUserCreds(
		context.Background(),
		serverauth.CredentialGenerationRequest{
			ProjectID: "client-user-123",
			ClientID:  "topic",
		},
	)
	assert.NoError(t, err)

	// Parse it back
	jwt, seed, err := serverauth.ParseCredsFile(creds)
	require.NoError(t, err)
	assert.NotEmpty(t, jwt)
	assert.NotEmpty(t, seed)
	assert.True(t, len(jwt) > 100, "JWT should be a substantial string")
	assert.True(t, len(seed) > 20, "Seed should be a substantial string")
}

func (s *NatsAuthSuite) TestParseCredsFileInvalidFormat() {
	t := s.T()

	// Invalid creds file
	_, _, err := serverauth.ParseCredsFile("invalid content")
	assert.Error(t, err)
}

func (s *NatsAuthSuite) TestParseCredsFileMissingSeed() {
	t := s.T()

	// Creds file missing seed section
	invalidCreds := `-----BEGIN NATS USER JWT-----
eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9
------END NATS USER JWT------
`
	_, _, err := serverauth.ParseCredsFile(invalidCreds)
	assert.Error(t, err)
}

func (s *NatsAuthSuite) TestNewNatsCredentialServiceEmptySeed() {
	t := s.T()

	_, err := serverauth.NewNatsCredentialService("", &testutils.NoopProjectService{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accountSeed is required")
}

func (s *NatsAuthSuite) TestNewNatsCredentialServiceInvalidSeed() {
	t := s.T()

	_, err := serverauth.NewNatsCredentialService("invalid-seed", &testutils.NoopProjectService{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse account seed")
}
