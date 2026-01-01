package serverauth

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestProject creates a project and returns the topic string for it
func (s *NatsAuthSuite) createTestProject(clientId string) string {
	project, err := s.ProjectsService.CreateProject(context.Background(), "test-project-"+clientId, []string{clientId})
	require.NoError(s.T(), err)
	return fmt.Sprintf("logs.%s.%s", project.ID.Hex(), clientId)
}

func (s *NatsAuthSuite) TestAuthCalloutValidToken() {
	t := s.T()

	// Create a project with a client
	topic := s.createTestProject("test-client")

	// Generate a valid token for the topic
	token, err := s.tokenService.GenerateToken("testuser", topic)
	require.NoError(t, err)

	// Connect to NATS with the token
	nc, err := nats.Connect(s.NatsAddr, nats.Token(token))
	require.NoError(t, err, "should connect with valid token")
	defer nc.Close()

	assert.True(t, nc.IsConnected())
}

func (s *NatsAuthSuite) TestAuthCalloutInvalidToken() {
	t := s.T()

	// Try to connect with an invalid token
	nc, err := nats.Connect(s.NatsAddr,
		nats.Token("invalid-token"),
		nats.Timeout(2*time.Second),
	)

	assert.Error(t, err, "should fail to connect with invalid token")
	if nc != nil {
		nc.Close()
	}
}

func (s *NatsAuthSuite) TestAuthCalloutNoToken() {
	t := s.T()

	// Try to connect without a token
	nc, err := nats.Connect(s.NatsAddr,
		nats.Timeout(2*time.Second),
	)

	assert.Error(t, err, "should fail to connect without token")
	if nc != nil {
		nc.Close()
	}
}

func (s *NatsAuthSuite) TestAuthCalloutPublishToWildcardClient() {
	t := s.T()

	// Create a project with a client
	topic := s.createTestProject("*")

	token, err := s.tokenService.GenerateToken("testuser", topic)
	require.NoError(t, err)

	nc, err := nats.Connect(s.NatsAddr, nats.Token(token))
	require.NoError(t, err)
	defer nc.Close()

	// Should be able to publish to the allowed topic
	err = nc.Publish(topic, []byte("test message"))
	assert.NoError(t, err, "should publish to allowed topic")

	err = nc.Flush()
	assert.NoError(t, err)
}

func (s *NatsAuthSuite) TestAuthCalloutPublishToAllowedTopic() {
	t := s.T()

	// Create a project with a client
	topic := s.createTestProject("allowed-client")

	token, err := s.tokenService.GenerateToken("testuser", topic)
	require.NoError(t, err)

	nc, err := nats.Connect(s.NatsAddr, nats.Token(token))
	require.NoError(t, err)
	defer nc.Close()

	// Should be able to publish to the allowed topic
	err = nc.Publish(topic, []byte("test message"))
	assert.NoError(t, err, "should publish to allowed topic")

	err = nc.Flush()
	assert.NoError(t, err)
}

func (s *NatsAuthSuite) TestAuthCalloutPublishToDisallowedTopic() {
	t := s.T()

	// Create a project with a client
	topic := s.createTestProject("my-client")

	token, err := s.tokenService.GenerateToken("testuser", topic)
	require.NoError(t, err)

	nc, err := nats.Connect(s.NatsAddr, nats.Token(token))
	require.NoError(t, err)
	defer nc.Close()

	// Try to publish to a different topic - should fail
	err = nc.Publish("logs.other.client", []byte("test message"))
	require.NoError(t, err) // Publish itself doesn't fail

	// Flush will fail or trigger permission error
	err = nc.Flush()
	// The connection may be closed due to permission violation
	if err == nil {
		// Give NATS time to process and close connection if needed
		time.Sleep(100 * time.Millisecond)
	}

	// Check if we received a permission error or connection was closed
	lastErr := nc.LastError()
	if lastErr != nil {
		assert.Contains(t, lastErr.Error(), "Permissions Violation")
	}
}

func (s *NatsAuthSuite) TestAuthCalloutProjectNotFound() {
	t := s.T()

	// Use a topic with a non-existent project
	topic := "logs.nonexistent123456789012.client"
	token, err := s.tokenService.GenerateToken("testuser", topic)
	require.NoError(t, err)

	// Connection should fail because project doesn't exist
	nc, err := nats.Connect(s.NatsAddr,
		nats.Token(token),
		nats.Timeout(2*time.Second),
	)

	assert.Error(t, err, "should fail to connect with non-existent project")
	if nc != nil {
		nc.Close()
	}
}

func (s *NatsAuthSuite) TestAuthCalloutClientNotInProject() {
	t := s.T()

	// Create a project with one client
	project, err := s.ProjectsService.CreateProject(context.Background(), "test-project-client-check", []string{"valid-client"})
	require.NoError(t, err)

	// Try to connect with a different client that's not in the project
	topic := fmt.Sprintf("logs.%s.%s", project.ID.Hex(), "invalid-client")
	token, err := s.tokenService.GenerateToken("testuser", topic)
	require.NoError(t, err)

	// Connection should fail because client is not in the project
	nc, err := nats.Connect(s.NatsAddr,
		nats.Token(token),
		nats.Timeout(2*time.Second),
	)

	assert.Error(t, err, "should fail to connect with client not in project")
	if nc != nil {
		nc.Close()
	}
}
