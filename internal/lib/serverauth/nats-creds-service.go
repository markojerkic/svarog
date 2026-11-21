package serverauth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"

	"github.com/markojerkic/svarog/internal/lib/svarogclient/config"
)

// ErrExpiryNotInFuture means the requested credential expiry is not strictly after the current time.
var ErrExpiryNotInFuture = errors.New("credential expiry must be in the future")

type ProjectLookup interface {
	ProjectExists(ctx context.Context, projectID, clientID string) bool
}

type NatsCredentialService struct {
	accountKeyPair   nkeys.KeyPair
	accountPublicKey string
	natsPublicAddr   string
	projectsService  ProjectLookup
}

func NewNatsCredentialService(
	accountSeed string,
	natsPublicAddr string,
	projectsService ProjectLookup) (*NatsCredentialService, error) {
	if projectsService == nil {
		return nil, errors.New("projectsService is required")
	}
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
		projectsService:  projectsService,
		natsPublicAddr:   natsPublicAddr,
	}, nil
}

type CredentialGenerationRequest struct {
	ProjectID string
	ClientID  string
	Expiry    *time.Time
}

func (s *NatsCredentialService) GenerateConnString(ctx context.Context, generationRequest CredentialGenerationRequest) (config.ClientConfig, error) {
	creds, err := s.GenerateUserCreds(ctx, generationRequest)
	if err != nil {
		return config.ClientConfig{}, err
	}
	base64EncodedCreds := base64.StdEncoding.EncodeToString([]byte(creds))

	return config.ClientConfig{
		Protocol:   "nats",
		ServerAddr: s.natsPublicAddr,
		Topic:      fmt.Sprintf("logs.%s.%s", generationRequest.ProjectID, generationRequest.ClientID),
		Creds:      base64EncodedCreds,
	}, nil
}

func (s *NatsCredentialService) GenerateUserCreds(ctx context.Context, generationRequest CredentialGenerationRequest) (string, error) {
	if exists := s.projectsService.ProjectExists(ctx, generationRequest.ProjectID, generationRequest.ClientID); !exists {
		return "", errors.New("project not found")
	}

	if generationRequest.Expiry != nil && !generationRequest.Expiry.After(time.Now()) {
		return "", ErrExpiryNotInFuture
	}

	userKp, err := nkeys.CreateUser()
	if err != nil {
		return "", fmt.Errorf("failed to create user key pair: %w", err)
	}
	defer userKp.Wipe()

	userPub, err := userKp.PublicKey()
	if err != nil {
		return "", fmt.Errorf("failed to get user public key: %w", err)
	}
	userSeed, err := userKp.Seed()
	if err != nil {
		return "", fmt.Errorf("failed to get user seed: %w", err)
	}

	claims := jwt.NewUserClaims(userPub)
	claims.Name = generationRequest.ProjectID
	claims.IssuerAccount = s.accountPublicKey

	if generationRequest.Expiry != nil {
		claims.Expires = generationRequest.Expiry.Unix()
	}

	claims.Permissions.Pub.Allow.Add(fmt.Sprintf("logs.%s.%s", generationRequest.ProjectID, generationRequest.ClientID))
	claims.Permissions.Sub.Allow.Add(config.PublisherInboxPrefix(userPub) + ".>")

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
