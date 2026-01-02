package serverauth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

// Credentials holds the NATS user JWT and private key seed
type Credentials struct {
	JWT  string
	Seed string
}

// NatsCredentialService generates NATS user credentials (JWT + seed)
// that can be used for decentralized authentication
type NatsCredentialService struct {
	accountKeyPair   nkeys.KeyPair
	accountPublicKey string
}

// NewNatsCredentialService creates a new credential service from an account seed
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

// GetAccountPublicKey returns the account public key for use in NATS server config
func (s *NatsCredentialService) GetAccountPublicKey() string {
	return s.accountPublicKey
}

// GenerateCredentials creates a NATS User JWT + seed for a specific topic.
// If expiry is nil, the credential never expires.
// The username is embedded in the JWT for identification purposes.
func (s *NatsCredentialService) GenerateCredentials(username, topic string, expiry *time.Duration) (*Credentials, error) {
	if topic == "" {
		return nil, errors.New("topic is required")
	}

	// Create user nkey pair
	userKp, err := nkeys.CreateUser()
	if err != nil {
		return nil, fmt.Errorf("failed to create user key pair: %w", err)
	}

	userPubKey, err := userKp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get user public key: %w", err)
	}

	userSeed, err := userKp.Seed()
	if err != nil {
		return nil, fmt.Errorf("failed to get user seed: %w", err)
	}

	// Create user claims with permissions
	userClaims := natsjwt.NewUserClaims(userPubKey)
	userClaims.Name = username
	userClaims.IssuerAccount = s.accountPublicKey

	// Set expiry if provided
	if expiry != nil {
		userClaims.Expires = time.Now().Add(*expiry).Unix()
	}

	// Grant publish permission only to the specified topic
	userClaims.Permissions.Pub.Allow.Add(topic)

	// Sign the JWT with the account key
	jwt, err := userClaims.Encode(s.accountKeyPair)
	if err != nil {
		return nil, fmt.Errorf("failed to encode user claims: %w", err)
	}

	return &Credentials{
		JWT:  jwt,
		Seed: string(userSeed),
	}, nil
}

// GenerateCredsFile returns the credentials in NATS .creds file format.
// This format can be used directly with nats.UserCredentials() or parsed manually.
func (s *NatsCredentialService) GenerateCredsFile(username, topic string, expiry *time.Duration) (string, error) {
	creds, err := s.GenerateCredentials(username, topic, expiry)
	if err != nil {
		return "", err
	}

	return FormatCredsFile(creds.JWT, creds.Seed), nil
}

// FormatCredsFile formats a JWT and seed into the standard NATS .creds file format
func FormatCredsFile(jwt, seed string) string {
	var sb strings.Builder
	sb.WriteString("-----BEGIN NATS USER JWT-----\n")
	sb.WriteString(jwt)
	sb.WriteString("\n------END NATS USER JWT------\n\n")
	sb.WriteString("************************* IMPORTANT *************************\n")
	sb.WriteString("NKEY Seed printed below can be used to sign and prove identity.\n")
	sb.WriteString("NKEYs are sensitive and should be treated as secrets.\n\n")
	sb.WriteString("-----BEGIN USER NKEY SEED-----\n")
	sb.WriteString(seed)
	sb.WriteString("\n------END USER NKEY SEED------\n")
	return sb.String()
}

// ParseCredsFile extracts the JWT and seed from a NATS .creds file format string
func ParseCredsFile(creds string) (jwt, seed string, err error) {
	// Extract JWT
	jwtStart := strings.Index(creds, "-----BEGIN NATS USER JWT-----")
	jwtEnd := strings.Index(creds, "------END NATS USER JWT------")
	if jwtStart == -1 || jwtEnd == -1 {
		return "", "", errors.New("invalid creds format: missing JWT section")
	}
	jwtContent := creds[jwtStart+len("-----BEGIN NATS USER JWT-----") : jwtEnd]
	jwt = strings.TrimSpace(jwtContent)

	// Extract seed
	seedStart := strings.Index(creds, "-----BEGIN USER NKEY SEED-----")
	seedEnd := strings.Index(creds, "------END USER NKEY SEED------")
	if seedStart == -1 || seedEnd == -1 {
		return "", "", errors.New("invalid creds format: missing seed section")
	}
	seedContent := creds[seedStart+len("-----BEGIN USER NKEY SEED-----") : seedEnd]
	seed = strings.TrimSpace(seedContent)

	return jwt, seed, nil
}
