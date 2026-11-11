package natsconn

import (
	"fmt"
	"time"

	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NatsConnectionConfig struct {
	NatsAddr               string
	User                   string // For user/password auth
	Password               string // For user/password auth
	JWT                    string // For JWT auth (alternative to user/password)
	Seed                   string // For JWT auth (alternative to user/password)
	EnableJetStream        bool
	DisableReconnectBuffer bool // live events must not replay from a stale local buffer
}

type NatsConnection struct {
	Conn      *nats.Conn
	JetStream jetstream.JetStream
}

func NewNatsConnection(cfg NatsConnectionConfig) (*NatsConnection, error) {
	var opts []nats.Option

	// Choose auth method: JWT or user/password
	if cfg.JWT != "" && cfg.Seed != "" {
		opts = append(opts, nats.UserJWTAndSeed(cfg.JWT, cfg.Seed))
	} else if cfg.User != "" && cfg.Password != "" {
		opts = append(opts, nats.UserInfo(cfg.User, cfg.Password))
	}

	opts = append(opts,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				slog.Error("NATS disconnected", "err", err)
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Info("NATS reconnected")
		}),
	)
	if cfg.DisableReconnectBuffer {
		opts = append(opts, nats.ReconnectBufSize(-1))
	}

	nc, err := nats.Connect(cfg.NatsAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	conn := &NatsConnection{
		Conn: nc,
	}

	// Only setup JetStream if enabled
	if cfg.EnableJetStream {
		js, err := jetstream.New(nc)
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("failed to create JetStream context: %w", err)
		}
		conn.JetStream = js
	}

	slog.Info("Connected to NATS", "addr", cfg.NatsAddr, "jetstream", cfg.EnableJetStream)

	return conn, nil
}

func (n *NatsConnection) Close() {
	if n.Conn != nil {
		n.Conn.Close()
	}
}
