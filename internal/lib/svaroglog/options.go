package svaroglog

import (
	"errors"
	"log/slog"
	"time"
)

// Option configures Setup. Defaults require only SVAROG_CONN_STRING.
type Option func(*setupConfig) error

type setupConfig struct {
	connectionString string
	instanceID       string
	level            slog.Leveler
	console          slog.Handler
	drainTimeout     time.Duration
}

// WithConnectionString overrides SVAROG_CONN_STRING.
func WithConnectionString(value string) Option {
	return func(config *setupConfig) error {
		config.connectionString = value
		return nil
	}
}

// WithInstanceID overrides the client's environment/hostname instance default.
func WithInstanceID(value string) Option {
	return func(config *setupConfig) error {
		config.instanceID = value
		return nil
	}
}

// WithLevel sets the minimum level sent to Svarog. Nil means INFO.
// The console's level is configured separately through its handler.
func WithLevel(value slog.Leveler) Option {
	return func(config *setupConfig) error {
		config.level = value
		return nil
	}
}

// WithConsole replaces the default INFO text handler on stderr. Nil disables it.
// Do not pass the initial slog.Default().Handler(), which can cause a log loop.
func WithConsole(value slog.Handler) Option {
	return func(config *setupConfig) error {
		config.console = value
		return nil
	}
}

// WithDrainTimeout sets a positive drain deadline. The default is five seconds.
func WithDrainTimeout(value time.Duration) Option {
	return func(config *setupConfig) error {
		if value <= 0 {
			return errors.New("logging drain timeout must be positive")
		}
		config.drainTimeout = value
		return nil
	}
}
