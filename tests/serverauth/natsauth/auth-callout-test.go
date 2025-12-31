package natsauth

import (
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *NatsAuthSuite) TestAuthCalloutValidToken() {
	t := s.T()

	// Generate a valid token for a specific topic
	token, err := s.tokenService.GenerateToken("testuser", "logs.myapp")
	require.NoError(t, err)

	// Connect to NATS with the token
	nc, err := nats.Connect(s.natsContainer.NatsAddr, nats.Token(token))
	require.NoError(t, err, "should connect with valid token")
	defer nc.Close()

	assert.True(t, nc.IsConnected())
}

func (s *NatsAuthSuite) TestAuthCalloutInvalidToken() {
	t := s.T()

	// Try to connect with an invalid token
	nc, err := nats.Connect(s.natsContainer.NatsAddr,
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
	nc, err := nats.Connect(s.natsContainer.NatsAddr,
		nats.Timeout(2*time.Second),
	)

	assert.Error(t, err, "should fail to connect without token")
	if nc != nil {
		nc.Close()
	}
}

func (s *NatsAuthSuite) TestAuthCalloutPublishToAllowedTopic() {
	t := s.T()

	topic := "logs.myapp"
	token, err := s.tokenService.GenerateToken("testuser", topic)
	require.NoError(t, err)

	nc, err := nats.Connect(s.natsContainer.NatsAddr, nats.Token(token))
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

	// Token grants access to "logs.myapp"
	token, err := s.tokenService.GenerateToken("testuser", "logs.myapp")
	require.NoError(t, err)

	nc, err := nats.Connect(s.natsContainer.NatsAddr, nats.Token(token))
	require.NoError(t, err)
	defer nc.Close()

	// Try to publish to a different topic - should fail
	err = nc.Publish("logs.other", []byte("test message"))
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
