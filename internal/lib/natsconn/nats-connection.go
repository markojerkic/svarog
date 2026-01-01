package natsconn

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"log/slog"
)

type JetStreamConfig struct {
	Name     string
	Subjects []string
}

type NatsConnectionConfig struct {
	NatsAddr        string
	User            string
	Password        string
	EnableJetStream bool
	JetStreamConfig JetStreamConfig
}

type NatsConnection struct {
	Conn      *nats.Conn
	JetStream jetstream.JetStream
}

func NewNatsConnection(cfg NatsConnectionConfig) (*NatsConnection, error) {
	nc, err := nats.Connect(cfg.NatsAddr,
		nats.UserInfo(cfg.User, cfg.Password),
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

		// Create the LOGS stream for log ingestion
		if err := conn.ensureLogsStream(cfg.JetStreamConfig); err != nil {
			slog.Warn("Failed to create JetStream LOGS stream", "err", err)
		}
	}

	slog.Info("Connected to NATS", "addr", cfg.NatsAddr, "jetstream", cfg.EnableJetStream)

	return conn, nil
}

func (n *NatsConnection) ensureLogsStream(cfg JetStreamConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := n.JetStream.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      cfg.Name,
		Subjects:  cfg.Subjects,
		Retention: jetstream.LimitsPolicy,
		MaxBytes:  1024 * 1024 * 1024, // 1GB
		MaxAge:    7 * 24 * time.Hour, // 7 days
		Storage:   jetstream.FileStorage,
		Discard:   jetstream.DiscardOld,
	})
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	slog.Info("JetStream LOGS stream ready")
	return nil
}

func (n *NatsConnection) Close() {
	if n.Conn != nil {
		n.Conn.Close()
	}
}
