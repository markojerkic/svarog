package svaroglog

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/markojerkic/svarog/internal/lib/svarogclient"
)

// ErrAlreadyConfigured means another Setup still owns a client in this process.
var ErrAlreadyConfigured = errors.New("Svarog logging is already configured")

var (
	setupMu     sync.Mutex
	setupActive bool
)

// Setup configures global slog and standard log using SVAROG_CONN_STRING or an
// explicit WithConnectionString option. Defaults are INFO, an INFO text console
// on stderr, and a five-second drain. It does not install signal handlers.
//
// Canceling lifetime starts shutdown independently of blocked logging calls.
// closeLogging also starts shutdown, waits for its result, and is safe to call
// repeatedly or concurrently. Call it before exiting to observe drain errors.
// Drain uses a fresh context and confirms broker ACKs, not database persistence.
//
// Only one Setup may be active. Cleanup restores the previous global logging
// state if the application still uses this setup's handler, including derivatives
// made with With/WithGroup. An independently replaced logger is left untouched.
// Applications should not reconfigure global logging concurrently with Setup or
// its cleanup.
func Setup(lifetime context.Context, options ...Option) (func() error, error) {
	if lifetime == nil {
		return nil, errors.New("logging lifetime context is required")
	}
	if err := lifetime.Err(); err != nil {
		return nil, err
	}
	config := setupConfig{
		connectionString: os.Getenv("SVAROG_CONN_STRING"),
		level:            slog.LevelInfo,
		console:          slog.NewTextHandler(os.Stderr, nil),
		drainTimeout:     5 * time.Second,
	}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("logging option must not be nil")
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	if config.connectionString == "" {
		return nil, errors.New("set SVAROG_CONN_STRING or use WithConnectionString")
	}

	setupMu.Lock()
	defer setupMu.Unlock()
	if setupActive {
		return nil, ErrAlreadyConfigured
	}
	client, err := svarogclient.New(svarogclient.Options{
		ConnectionString: config.connectionString,
		InstanceID:       config.instanceID,
	})
	if err != nil {
		return nil, err
	}

	previous := slog.Default()
	output, flags, prefix := log.Writer(), log.Flags(), log.Prefix()
	handler := &setupHandler{
		Handler: NewHandler(client, Options{Level: config.level, Console: config.console}),
		client:  client,
	}
	slog.SetDefault(slog.New(handler))
	level := slog.SetLogLoggerLevel(slog.LevelInfo)
	setupActive = true

	var once sync.Once
	var result error
	closeLogging := func() error {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), config.drainTimeout)
			defer cancel()
			result = client.Shutdown(ctx)

			setupMu.Lock()
			defer setupMu.Unlock()
			if current, ok := slog.Default().Handler().(*setupHandler); ok && current.client == client {
				slog.SetDefault(previous)
				slog.SetLogLoggerLevel(level)
				log.SetOutput(output)
				log.SetFlags(flags)
				log.SetPrefix(prefix)
			}
			setupActive = false
		})
		return result
	}
	stop := context.AfterFunc(lifetime, func() { _ = closeLogging() })
	// Unregister explicit cleanup so a long-lived context does not retain it.
	return func() error {
		stop()
		return closeLogging()
	}, nil
}

// setupHandler preserves ownership through derived global loggers.
type setupHandler struct {
	slog.Handler
	client *svarogclient.Client
}

func (handler *setupHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &setupHandler{Handler: handler.Handler.WithAttrs(attrs), client: handler.client}
}

func (handler *setupHandler) WithGroup(name string) slog.Handler {
	return &setupHandler{Handler: handler.Handler.WithGroup(name), client: handler.client}
}
