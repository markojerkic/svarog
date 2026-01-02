package serverauth

import (
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

type NatsCredentialService struct {
	accountKeyPair   nkeys.KeyPair
	accountPublicKey string
}

func NewNatsCredentialService(accountSeed string) (*NatsCredentialService, error) {
	if accountSeed == "" {
		return nil, errors.New("accountSeed is required")
	}

	accountKp, err := nkeys.FromSeed([]byte(accountSeed))
	if err != nil {
		return nil, fmt.Errorf("failed to parse account seed: %w", err)
	}

	accountPubKey, err := accountKp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get account public key: %w", err)
	}

	return &NatsCredentialService{
		accountKeyPair:   accountKp,
		accountPublicKey: accountPubKey,
	}, nil
}

func (s *NatsCredentialService) GenerateUserCreds(username string, pubAllowed []string, subAllowed []string, expiry *time.Duration) (string, error) {
	userKp, err := nkeys.CreateUser()
	if err != nil {
		return "", fmt.Errorf("failed to create user key pair: %w", err)
	}

	userPub, _ := userKp.PublicKey()
	userSeed, _ := userKp.Seed()

	claims := jwt.NewUserClaims(userPub)
	claims.Name = username
	claims.IssuerAccount = s.accountPublicKey

	if expiry != nil {
		claims.Expires = time.Now().Add(*expiry).Unix()
	}

	// Publish Permissions
	if len(pubAllowed) > 0 {
		claims.Permissions.Pub.Allow.Add(pubAllowed...)
	}
	// Subscribe Permissions
	if len(subAllowed) > 0 {
		claims.Permissions.Sub.Allow.Add(subAllowed...)
	}

	userJwt, err := claims.Encode(s.accountKeyPair)
	if err != nil {
		return "", fmt.Errorf("failed to encode user claims: %w", err)
	}

	credsBytes, err := jwt.FormatUserConfig(userJwt, userSeed)
	if err != nil {
		return "", fmt.Errorf("failed to format creds: %w", err)
	}

	return string(credsBytes), nil
}
