package natsauth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestNatsAuthSuite(t *testing.T) {
	suite.Run(t, new(NatsAuthSuite))
}

func (s *NatsAuthSuite) TestGenerateToken() {
	t := s.T()

	token, err := s.authHandler.GenerateToken("testuser", "logs.myapp")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func (s *NatsAuthSuite) TestGenerateTokenEmptyTopicFails() {
	t := s.T()

	token, err := s.authHandler.GenerateToken("testuser", "")
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "topic is required")
}

func (s *NatsAuthSuite) TestValidateJWT() {
	t := s.T()

	token, err := s.authHandler.GenerateToken("testuser", "logs.myapp")
	require.NoError(t, err)

	claims, err := s.authHandler.ValidateJWT(token)
	require.NoError(t, err)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "logs.myapp", claims.Topic)
}

func (s *NatsAuthSuite) TestValidateJWTEmptyToken() {
	t := s.T()

	claims, err := s.authHandler.ValidateJWT("")
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "token is empty")
}

func (s *NatsAuthSuite) TestValidateJWTInvalidToken() {
	t := s.T()

	claims, err := s.authHandler.ValidateJWT("invalid.token.here")
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func (s *NatsAuthSuite) TestValidateJWTWrongSecret() {
	t := s.T()

	// Create a token with a different secret
	claims := serverauth.NatsAuthClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Username: "testuser",
		Topic:    "logs.myapp",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("wrong-secret"))
	require.NoError(t, err)

	result, err := s.authHandler.ValidateJWT(tokenString)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func (s *NatsAuthSuite) TestValidateJWTExpiredToken() {
	t := s.T()

	// Create an expired token
	claims := serverauth.NatsAuthClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		Username: "testuser",
		Topic:    "logs.myapp",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-jwt-secret"))
	require.NoError(t, err)

	result, err := s.authHandler.ValidateJWT(tokenString)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func (s *NatsAuthSuite) TestGenerateAndValidateRoundTrip() {
	t := s.T()

	username := "myuser"
	topic := "logs.service.production"

	token, err := s.authHandler.GenerateToken(username, topic)
	require.NoError(t, err)

	claims, err := s.authHandler.ValidateJWT(token)
	require.NoError(t, err)

	assert.Equal(t, username, claims.Username)
	assert.Equal(t, topic, claims.Topic)
	assert.True(t, claims.ExpiresAt.After(time.Now()))
}
