package natsconn

import (
	"context"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"
)

type ExternalConfig struct {
	Addr     string `env:"NATS_ADDR"`
	UserJWT  string `env:"NATS_SERVER_USER_JWT"`
	UserSeed string `env:"NATS_SERVER_USER_SEED"`
}

type External struct {
	config ExternalConfig
}

func NewExternal(cfg ExternalConfig) *External {
	return &External{config: cfg}
}

func (e *External) Start(context.Context) error {
	for _, setting := range []struct{ name, value string }{
		{"NATS_ADDR", e.config.Addr},
		{"NATS_SERVER_USER_JWT", e.config.UserJWT},
		{"NATS_SERVER_USER_SEED", e.config.UserSeed},
	} {
		if strings.TrimSpace(setting.value) == "" {
			return fmt.Errorf("%s is required for external NATS", setting.name)
		}
	}
	return nil
}

func (e *External) Connect(opts ConnectOptions) (*NatsConnection, error) {
	return Connect(e.config.Addr, opts, nats.UserJWTAndSeed(e.config.UserJWT, e.config.UserSeed))
}

func (e *External) Shutdown(context.Context) error { return nil }
