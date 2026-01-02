package serverauth

import (
	"fmt"

	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

// NatsAuthSetup contains all the keys and JWTs needed for NATS decentralized auth.
// Use GenerateNatsAuthSetup() once to generate these values, then store them in
// environment variables for production use.
type NatsAuthSetup struct {
	// Operator
	OperatorKeyPair   nkeys.KeyPair
	OperatorPublicKey string
	OperatorJWT       string

	// APP Account (for both clients and server - has JetStream)
	AccountKeyPair   nkeys.KeyPair
	AccountPublicKey string
	AccountSeed      string
	AccountJWT       string

	// System account (for NATS internal monitoring, no JetStream)
	SystemAccountKeyPair   nkeys.KeyPair
	SystemAccountPublicKey string
	SystemAccountJWT       string

	// Server user credentials (in APP account, full permissions for server)
	ServerUserJWT  string
	ServerUserSeed string
}

// GenerateNatsAuthSetup creates a complete set of operator, account, and system account
// keys and JWTs for NATS decentralized authentication.
//
// Run this once and store the output in environment variables:
//   - NATS_OPERATOR_JWT
//   - NATS_ACCOUNT_PUBLIC_KEY
//   - NATS_ACCOUNT_SEED (for signing client user JWTs)
//   - NATS_ACCOUNT_JWT
//   - NATS_SYSTEM_ACCOUNT_PUBLIC_KEY
//   - NATS_SYSTEM_ACCOUNT_JWT
//   - NATS_SERVER_USER_JWT (server connects with this)
//   - NATS_SERVER_USER_SEED (server connects with this)
func GenerateNatsAuthSetup() (*NatsAuthSetup, error) {
	setup := &NatsAuthSetup{}

	// Create operator key pair
	operatorKp, err := nkeys.CreateOperator()
	if err != nil {
		return nil, fmt.Errorf("failed to create operator key pair: %w", err)
	}
	setup.OperatorKeyPair = operatorKp

	operatorPubKey, err := operatorKp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get operator public key: %w", err)
	}
	setup.OperatorPublicKey = operatorPubKey

	// Create operator JWT
	operatorClaims := natsjwt.NewOperatorClaims(operatorPubKey)
	operatorClaims.Name = "svarog-operator"
	operatorJwt, err := operatorClaims.Encode(operatorKp)
	if err != nil {
		return nil, fmt.Errorf("failed to encode operator claims: %w", err)
	}
	setup.OperatorJWT = operatorJwt

	// Create APP account key pair (for signing user JWTs, has JetStream)
	accountKp, err := nkeys.CreateAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to create account key pair: %w", err)
	}
	setup.AccountKeyPair = accountKp

	accountPubKey, err := accountKp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get account public key: %w", err)
	}
	setup.AccountPublicKey = accountPubKey

	accountSeed, err := accountKp.Seed()
	if err != nil {
		return nil, fmt.Errorf("failed to get account seed: %w", err)
	}
	setup.AccountSeed = string(accountSeed)

	// Create APP account JWT (signed by operator) - with JetStream
	accountClaims := natsjwt.NewAccountClaims(accountPubKey)
	accountClaims.Name = "APP"
	accountClaims.Limits.JetStreamLimits.Consumer = -1
	accountClaims.Limits.JetStreamLimits.Streams = -1
	accountClaims.Limits.JetStreamLimits.MemoryStorage = -1
	accountClaims.Limits.JetStreamLimits.DiskStorage = -1
	accountJwt, err := accountClaims.Encode(operatorKp)
	if err != nil {
		return nil, fmt.Errorf("failed to encode account claims: %w", err)
	}
	setup.AccountJWT = accountJwt

	// Create system account key pair (for NATS internal use, no JetStream)
	systemAccountKp, err := nkeys.CreateAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to create system account key pair: %w", err)
	}
	setup.SystemAccountKeyPair = systemAccountKp

	systemAccountPubKey, err := systemAccountKp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get system account public key: %w", err)
	}
	setup.SystemAccountPublicKey = systemAccountPubKey

	// Create system account JWT (signed by operator) - NO JetStream
	systemAccountClaims := natsjwt.NewAccountClaims(systemAccountPubKey)
	systemAccountClaims.Name = "SYS"
	systemAccountJwt, err := systemAccountClaims.Encode(operatorKp)
	if err != nil {
		return nil, fmt.Errorf("failed to encode system account claims: %w", err)
	}
	setup.SystemAccountJWT = systemAccountJwt

	// Create server user credentials (in APP account, full permissions)
	serverUserKp, err := nkeys.CreateUser()
	if err != nil {
		return nil, fmt.Errorf("failed to create server user key pair: %w", err)
	}

	serverUserPubKey, err := serverUserKp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get server user public key: %w", err)
	}

	serverUserSeed, err := serverUserKp.Seed()
	if err != nil {
		return nil, fmt.Errorf("failed to get server user seed: %w", err)
	}
	setup.ServerUserSeed = string(serverUserSeed)

	// Create server user JWT (signed by APP account, full permissions)
	serverUserClaims := natsjwt.NewUserClaims(serverUserPubKey)
	serverUserClaims.Name = "svarog-server"
	serverUserClaims.IssuerAccount = accountPubKey
	// Server has full pub/sub permissions
	serverUserClaims.Permissions.Pub.Allow.Add(">")
	serverUserClaims.Permissions.Sub.Allow.Add(">")

	serverUserJwt, err := serverUserClaims.Encode(accountKp)
	if err != nil {
		return nil, fmt.Errorf("failed to encode server user claims: %w", err)
	}
	setup.ServerUserJWT = serverUserJwt

	return setup, nil
}

// GetServerUserCredsFile returns the server user credentials in .creds file format
func (s *NatsAuthSetup) GetServerUserCredsFile() string {
	return FormatCredsFile(s.ServerUserJWT, s.ServerUserSeed)
}

// GenerateNatsConfig returns a NATS server configuration string for testing.
// For production, use the static nats-server.conf with env vars.
func (s *NatsAuthSetup) GenerateNatsConfig(wsPort string) string {
	return fmt.Sprintf(`port: 4222

jetstream {
  store_dir: "/data/jetstream"
}

websocket {
  port: %s
  no_tls: true
}

operator: %s

resolver: MEMORY
resolver_preload: {
  %s: %s
  %s: %s
}

system_account: %s
`, wsPort, s.OperatorJWT, s.AccountPublicKey, s.AccountJWT, s.SystemAccountPublicKey, s.SystemAccountJWT,
		s.SystemAccountPublicKey)
}

// PrintEnvVars prints all environment variables needed for nats-server.conf
func (s *NatsAuthSetup) PrintEnvVars() string {
	return fmt.Sprintf(`# NATS Decentralized JWT Auth Configuration
# Generated by: go run cmd/nats-setup/main.go
# Copy these to your .env file

# Operator JWT (root of trust)
NATS_OPERATOR_JWT=%s

# APP Account (for client and server user JWTs, has JetStream)
NATS_ACCOUNT_PUBLIC_KEY=%s
NATS_ACCOUNT_SEED=%s
NATS_ACCOUNT_JWT=%s

# System Account (for NATS internal monitoring, no JetStream)
NATS_SYSTEM_ACCOUNT_PUBLIC_KEY=%s
NATS_SYSTEM_ACCOUNT_JWT=%s

# Server User Credentials (in APP account, for server to connect to NATS)
NATS_SERVER_USER_JWT=%s
NATS_SERVER_USER_SEED=%s
`,
		s.OperatorJWT,
		s.AccountPublicKey,
		s.AccountSeed,
		s.AccountJWT,
		s.SystemAccountPublicKey,
		s.SystemAccountJWT,
		s.ServerUserJWT,
		s.ServerUserSeed,
	)
}
