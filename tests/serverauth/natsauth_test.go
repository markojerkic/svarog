package serverauth

import (
	"encoding/base64"
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

	creds, err := s.NatsCredsService.GenerateUserCreds(
		"client-user-123",
		[]string{"topic"},
		[]string{"_INBOX.>"},
		nil,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, creds)
}

func (s *NatsAuthSuite) TestGenerateCredentialsWithExpiry() {
	t := s.T()
	expiry := 24 * time.Hour

	creds, err := s.NatsCredsService.GenerateUserCreds(
		"client-user-123",
		[]string{"topic"},
		[]string{"_INBOX.>"},
		&expiry,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, creds)
}

func (s *NatsAuthSuite) TestGenerateCredsFile() {
	t := s.T()

	creds, err := s.NatsCredsService.GenerateUserCreds(
		"client-user-123",
		[]string{"topic"},
		[]string{"_INBOX.>"},
		nil,
	)

	assert.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(creds)
	assert.NoError(t, err, "should decode creds")

	assert.Contains(t, decoded, "-----BEGIN NATS USER JWT-----")
	assert.Contains(t, decoded, "------END NATS USER JWT------")
	assert.Contains(t, decoded, "-----BEGIN USER NKEY SEED-----")
	assert.Contains(t, decoded, "------END USER NKEY SEED------")
}

func (s *NatsAuthSuite) TestParseCredsFile() {
	t := s.T()

	creds, err := s.NatsCredsService.GenerateUserCreds(
		"client-user-123",
		[]string{"topic"},
		[]string{"_INBOX.>"},
		nil,
	)

	assert.NoError(t, err)
	credsFile, err := base64.StdEncoding.DecodeString(creds)
	assert.NoError(t, err, "should decode creds")

	// Parse it back
	jwt, seed, err := serverauth.ParseCredsFile(string(credsFile))
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
