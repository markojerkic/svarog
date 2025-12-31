package testutils

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/nats-io/nkeys"
	"github.com/testcontainers/testcontainers-go/modules/nats"
)

const (
	DefaultSystemUser     = "system"
	DefaultSystemPassword = "password"
	DefaultAppUser        = "app"
	DefaultAppPassword    = "apppass"
	DefaultJWTSecret      = "test-jwt-secret"
	DefaultNatsWsPort     = "9222"
)

// NATSTestContainer holds the NATS testcontainer and its connection details
type NATSTestContainer struct {
	Container    *nats.NATSContainer
	NatsAddr     string
	NatsConn     *serverauth.NatsConnection
	TokenService *serverauth.TokenService
	AuthHandler  *serverauth.NatsAuthCalloutHandler
	IssuerSeed   string
}

// NATSTestConfig holds configuration for the NATS test container
type NATSTestConfig struct {
	SystemUser     string
	SystemPassword string
	AppUser        string
	AppPassword    string
	JWTSecret      string
	NatsWsPort     string
	ConfigPath     string // Path to nats-server.conf
}

// DefaultNATSTestConfig returns the default configuration
func DefaultNATSTestConfig() NATSTestConfig {
	return NATSTestConfig{
		SystemUser:     DefaultSystemUser,
		SystemPassword: DefaultSystemPassword,
		AppUser:        DefaultAppUser,
		AppPassword:    DefaultAppPassword,
		JWTSecret:      DefaultJWTSecret,
		NatsWsPort:     DefaultNatsWsPort,
		ConfigPath:     "nats-server.conf",
	}
}

// NewNATSTestContainer creates and starts a NATS testcontainer with auth callout
func NewNATSTestContainer(ctx context.Context, config NATSTestConfig) (*NATSTestContainer, error) {
	// Generate NATS issuer key pair
	issuerKp, err := nkeys.CreateAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to create issuer key pair: %w", err)
	}

	issuerSeed, err := issuerKp.Seed()
	if err != nil {
		return nil, fmt.Errorf("failed to get issuer seed: %w", err)
	}

	issuerPublicKey, err := issuerKp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get issuer public key: %w", err)
	}

	// Read and substitute variables in nats-server.conf
	natsConfig, err := loadNatsConfig(config, issuerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load NATS config: %w", err)
	}

	// Start NATS container with config
	container, err := nats.Run(ctx, "nats:latest",
		nats.WithConfigFile(strings.NewReader(natsConfig)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start NATS container: %w", err)
	}

	natsAddr, err := container.ConnectionString(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get NATS connection string: %w", err)
	}

	// Create token service
	tokenService, err := serverauth.NewTokenService(config.JWTSecret)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create token service: %w", err)
	}

	// Create NATS connection for auth callout (SYSTEM account)
	natsConn, err := serverauth.NewNatsConnection(serverauth.NatsConnectionConfig{
		NatsAddr:        natsAddr,
		User:            config.SystemUser,
		Password:        config.SystemPassword,
		EnableJetStream: true,
	})
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create NATS connection: %w", err)
	}

	// Create auth handler with explicit config
	authHandler, err := serverauth.NewNatsAuthCalloutHandler(serverauth.NatsAuthConfig{
		IssuerSeed: string(issuerSeed),
	}, natsConn.Conn, tokenService)
	if err != nil {
		natsConn.Close()
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create auth handler: %w", err)
	}

	// Start the auth callout handler
	if err := authHandler.Run(); err != nil {
		natsConn.Close()
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to start auth callout handler: %w", err)
	}

	return &NATSTestContainer{
		Container:    container,
		NatsAddr:     natsAddr,
		NatsConn:     natsConn,
		TokenService: tokenService,
		AuthHandler:  authHandler,
		IssuerSeed:   string(issuerSeed),
	}, nil
}

func loadNatsConfig(config NATSTestConfig, issuerPublicKey string) (string, error) {
	configBytes, err := os.ReadFile(config.ConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to read nats-server.conf: %w", err)
	}

	configStr := string(configBytes)

	// Substitute environment variables
	replacer := strings.NewReplacer(
		"$NATS_WS_PORT", config.NatsWsPort,
		"$NATS_SYSTEM_USER", config.SystemUser,
		"$NATS_SYSTEM_PASSWORD", config.SystemPassword,
		"$NATS_APP_USER", config.AppUser,
		"$NATS_APP_PASSWORD", config.AppPassword,
		"$NATS_ISSUER_PUBLIC_KEY", issuerPublicKey,
	)

	return replacer.Replace(configStr), nil
}

// Terminate cleans up the NATS testcontainer
func (tc *NATSTestContainer) Terminate(ctx context.Context) error {
	if tc.NatsConn != nil {
		tc.NatsConn.Close()
	}
	if tc.Container != nil {
		return tc.Container.Terminate(ctx)
	}
	return nil
}
