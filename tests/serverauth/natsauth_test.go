package serverauth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/types"
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
	projectID, clientID, _ := s.createTestProject("generate-client")

	creds, err := s.NatsCredsService.GenerateUserCreds(
		context.Background(),
		serverauth.CredentialGenerationRequest{
			ProjectID: projectID,
			ClientID:  clientID,
		},
	)
	require.NoError(t, err, "generate credentials for actual project and client UUIDs")
	assert.NotEmpty(t, creds, "valid project/client UUIDs should produce a credentials file")
}

func (s *NatsAuthSuite) TestGenerateCredentialsRejectsInvalidMembership() {
	t := s.T()
	ctx := context.Background()
	projectID, clientID, _ := s.createTestProject("membership-client")
	otherProjectID, otherClientID, _ := s.createTestProject("other-client")
	for _, tc := range []struct {
		name      string
		projectID string
		clientID  string
	}{
		{"client display name", projectID, "membership-client"},
		{"missing client UUID", projectID, uuid.NewString()},
		{"client from another project", projectID, otherClientID},
		{"project from another client", otherProjectID, clientID},
		{"wildcard for missing project", uuid.NewString(), "*"},
	} {
		creds, err := s.NatsCredsService.GenerateUserCreds(ctx, serverauth.CredentialGenerationRequest{ProjectID: tc.projectID, ClientID: tc.clientID})
		assert.Error(t, err, "%s must not be authorized for credentials", tc.name)
		assert.Empty(t, creds, "%s must not receive a credentials file", tc.name)
	}

	_, err := s.ProjectsService.UpdateProject(ctx, uuid.MustParse(projectID), "test-project-membership-client", nil)
	require.NoError(t, err, "deactivate the real client before credential validation")
	creds, err := s.NatsCredsService.GenerateUserCreds(ctx, serverauth.CredentialGenerationRequest{ProjectID: projectID, ClientID: clientID})
	assert.Error(t, err, "inactive client UUID must not receive new credentials")
	assert.Empty(t, creds, "inactive client UUID must not produce a credentials file")
}

func (s *NatsAuthSuite) TestGenerateConnStringUsesClientUUID() {
	t := s.T()
	projectID, clientID, subject := s.createTestProject("connection-client")
	config, err := s.NatsCredsService.GenerateConnString(context.Background(), serverauth.CredentialGenerationRequest{ProjectID: projectID, ClientID: clientID})
	require.NoError(t, err, "generate connection configuration for real UUID fixtures")
	assert.Equal(t, subject, config.Topic, "connection topic must contain project and client UUIDs rather than display names")
	assert.NotEmpty(t, config.Creds, "connection configuration must include encoded credentials")
}

func (s *NatsAuthSuite) TestGenerateCredentialsWithExpiry() {
	t := s.T()
	projectID, clientID, _ := s.createTestProject("expiry-client")
	expiry := 24 * time.Hour
	expiryTime := time.Now().Add(expiry)

	creds, err := s.NatsCredsService.GenerateUserCreds(
		context.Background(),
		serverauth.CredentialGenerationRequest{
			ProjectID: projectID,
			ClientID:  clientID,
			Expiry:    types.NullableDate{Time: expiryTime, Valid: true},
		},
	)
	require.NoError(t, err, "generate expiring credentials for actual project and client UUIDs")
	assert.NotEmpty(t, creds, "valid project/client UUIDs should produce expiring credentials")
}

func (s *NatsAuthSuite) TestGenerateCredsFile() {
	t := s.T()
	projectID, clientID, _ := s.createTestProject("file-client")

	creds, err := s.NatsCredsService.GenerateUserCreds(
		context.Background(),
		serverauth.CredentialGenerationRequest{
			ProjectID: projectID,
			ClientID:  clientID,
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
	projectID, clientID, _ := s.createTestProject("parse-client")

	creds, err := s.NatsCredsService.GenerateUserCreds(
		context.Background(),
		serverauth.CredentialGenerationRequest{
			ProjectID: projectID,
			ClientID:  clientID,
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

	_, err := serverauth.NewNatsCredentialService("", "", &testutils.NoopProjectService{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accountSeed is required")
}

func (s *NatsAuthSuite) TestNewNatsCredentialServiceInvalidSeed() {
	t := s.T()

	_, err := serverauth.NewNatsCredentialService("invalid-seed", "", &testutils.NoopProjectService{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse account seed")
}
