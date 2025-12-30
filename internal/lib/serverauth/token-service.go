package serverauth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenService struct {
	jwtSecret []byte
}

func NewTokenService(jwtSecret string) (*TokenService, error) {
	if jwtSecret == "" {
		return nil, errors.New("jwtSecret is required")
	}

	return &TokenService{
		jwtSecret: []byte(jwtSecret),
	}, nil
}

// GenerateToken creates a signed JWT token that grants access to a specific topic.
func (t *TokenService) GenerateToken(username, topic string) (string, error) {
	if topic == "" {
		return "", errors.New("topic is required")
	}

	claims := NatsAuthClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
		Username: username,
		Topic:    topic,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.jwtSecret)
}

// ValidateJWT validates the JWT token and returns the claims if valid.
func (t *TokenService) ValidateJWT(tokenString string) (*NatsAuthClaims, error) {
	if tokenString == "" {
		return nil, errors.New("token is empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &NatsAuthClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return t.jwtSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*NatsAuthClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}
