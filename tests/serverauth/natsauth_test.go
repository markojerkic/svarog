package serverauth

import (
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestNatsAuthSuite(t *testing.T) {
	suite.Run(t, new(NatsAuthSuite))
}

func (s *NatsAuthSuite) TestGenerateCredentials() {
	t := s.T()

	creds, err := s.credentialService.GenerateCredentials("testuser", "logs.myapp.client", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, creds.JWT)
	assert.NotEmpty(t, creds.Seed)
}

func (s *NatsAuthSuite) TestGenerateCredentialsEmptyTopicFails() {
	t := s.T()

	creds, err := s.credentialService.GenerateCredentials("testuser", "", nil)
	assert.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "topic is required")
}

func (s *NatsAuthSuite) TestGenerateCredentialsWithExpiry() {
	t := s.T()

	expiry := 24 * time.Hour
	creds, err := s.credentialService.GenerateCredentials("testuser", "logs.myapp.client", &expiry)
	require.NoError(t, err)
	assert.NotEmpty(t, creds.JWT)
	assert.NotEmpty(t, creds.Seed)
}

func (s *NatsAuthSuite) TestGenerateCredsFile() {
	t := s.T()

	credsFile, err := s.credentialService.GenerateCredsFile("testuser", "logs.myapp.client", nil)
	require.NoError(t, err)
	assert.Contains(t, credsFile, "-----BEGIN NATS USER JWT-----")
	assert.Contains(t, credsFile, "------END NATS USER JWT------")
	assert.Contains(t, credsFile, "-----BEGIN USER NKEY SEED-----")
	assert.Contains(t, credsFile, "------END USER NKEY SEED------")
}

func (s *NatsAuthSuite) TestParseCredsFile() {
	t := s.T()

	// Generate a creds file
	credsFile, err := s.credentialService.GenerateCredsFile("testuser", "logs.myapp.client", nil)
	require.NoError(t, err)

	// Parse it back
	jwt, seed, err := serverauth.ParseCredsFile(credsFile)
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
	assert.Contains(t, err.Error(), "invalid creds format")
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
	assert.Contains(t, err.Error(), "missing seed section")
}

func (s *NatsAuthSuite) TestFormatCredsFile() {
	t := s.T()

	jwt := "test-jwt-token"
	seed := "SUAM-test-seed"

	credsFile := serverauth.FormatCredsFile(jwt, seed)

	assert.Contains(t, credsFile, "-----BEGIN NATS USER JWT-----")
	assert.Contains(t, credsFile, jwt)
	assert.Contains(t, credsFile, "------END NATS USER JWT------")
	assert.Contains(t, credsFile, "-----BEGIN USER NKEY SEED-----")
	assert.Contains(t, credsFile, seed)
	assert.Contains(t, credsFile, "------END USER NKEY SEED------")
}

func (s *NatsAuthSuite) TestGenerateAndParseRoundTrip() {
	t := s.T()

	username := "myuser"
	topic := "logs.service.production"

	// Generate creds file
	credsFile, err := s.credentialService.GenerateCredsFile(username, topic, nil)
	require.NoError(t, err)

	// Parse it
	jwt, seed, err := serverauth.ParseCredsFile(credsFile)
	require.NoError(t, err)

	// Both should be non-empty
	assert.NotEmpty(t, jwt)
	assert.NotEmpty(t, seed)
}

func (s *NatsAuthSuite) TestGetAccountPublicKey() {
	t := s.T()

	pubKey := s.credentialService.GetAccountPublicKey()
	assert.NotEmpty(t, pubKey)
	// Account public keys start with 'A'
	assert.True(t, pubKey[0] == 'A', "Account public key should start with 'A'")
}

func (s *NatsAuthSuite) TestNewNatsCredentialServiceEmptySeed() {
	t := s.T()

	_, err := serverauth.NewNatsCredentialService("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accountSeed is required")
}

func (s *NatsAuthSuite) TestNewNatsCredentialServiceInvalidSeed() {
	t := s.T()

	_, err := serverauth.NewNatsCredentialService("invalid-seed")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse account seed")
}
