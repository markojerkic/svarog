package natsclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/cmd/client/config"
	"github.com/markojerkic/svarog/internal/commontypes"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NatsClient struct {
	config    config.ClientConfig
	jetstream jetstream.JetStream
	logLines  chan *commontypes.LogLineDto
}

func NewNatsClient(cfg config.ClientConfig, logLines chan *commontypes.LogLineDto) NatsClient {
	js, err := connectNats(cfg)
	if err != nil {
		log.Fatal("Failed to connect to NATS", "err", err)
	}

	return NatsClient{
		config:    cfg,
		jetstream: js,
		logLines:  logLines,
	}
}

func (n *NatsClient) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Debug("NATS client context done")
			return
		case logLine := <-n.logLines:
			data, err := json.Marshal(logLine)
			if err != nil {
				log.Error("Failed to marshal log line", "err", err)
				continue
			}

			if _, err := n.jetstream.Publish(ctx, n.config.Topic, data); err != nil {
				log.Error("Failed to publish log line", "err", err)
			}
		}
	}
}

func connectNats(config config.ClientConfig) (jetstream.JetStream, error) {
	nc, err := nats.Connect(config.GetNatsUrl(),
		nats.Token(config.Token),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Error("Disconnected from NATS", "err", err)
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Debug("Reconnected to NATS")
		}),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			log.Error("NATS error", "err", err)
		}),
	)
	if err != nil {
		return nil, err
	}

	return jetstream.New(nc)
}
