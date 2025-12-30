package serverauth

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NatsConnectionConfig struct {
	NatsAddr       string
	SystemUser     string
	SystemPassword string
}

type NatsConnection struct {
	Conn      *nats.Conn
	JetStream jetstream.JetStream
}

func NewNatsConnection(cfg NatsConnectionConfig) (*NatsConnection, error) {
	nc, err := nats.Connect(cfg.NatsAddr,
		nats.UserInfo(cfg.SystemUser, cfg.SystemPassword),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Error("NATS disconnected", "err", err)
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Info("NATS reconnected")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	conn := &NatsConnection{
		Conn:      nc,
		JetStream: js,
	}

	if err := conn.ensureLogsStream(); err != nil {
		log.Warn("Failed to create JetStream LOGS stream", "err", err)
	}

	return conn, nil
}

func (n *NatsConnection) ensureLogsStream() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := n.JetStream.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      "LOGS",
		Subjects:  []string{"logs.>"},
		Retention: jetstream.LimitsPolicy,
		MaxBytes:  1024 * 1024 * 1024, // 1GB
		MaxAge:    7 * 24 * time.Hour, // 7 days
		Storage:   jetstream.FileStorage,
		Discard:   jetstream.DiscardOld,
	})
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	log.Info("JetStream LOGS stream ready")
	return nil
}

func (n *NatsConnection) Close() {
	if n.Conn != nil {
		n.Conn.Close()
	}
}
