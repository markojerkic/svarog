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
	config   config.ClientConfig
	js       jetstream.JetStream
	logLines chan *commontypes.LogLineDto
}

func NewNatsClient(cfg config.ClientConfig, logLines chan *commontypes.LogLineDto) NatsClient {
	_, js := connectNats(cfg)

	return NatsClient{
		config:   cfg,
		js:       js,
		logLines: logLines,
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

			if _, err := n.js.Publish(ctx, n.config.Topic, data); err != nil {
				log.Error("Failed to publish log line", "err", err)
			}
		}
	}
}

func connectNats(cfg config.ClientConfig) (*nats.Conn, jetstream.JetStream) {
	opts := []nats.Option{
		nats.Token(cfg.Token),
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
	}

	for {
		nc, err := nats.Connect(cfg.GetNatsUrl(), opts...)
		if err == nil {
			log.Info("Connected to NATS", "url", cfg.GetNatsUrl())

			js, err := jetstream.New(nc)
			if err != nil {
				log.Error("Failed to create JetStream context, retrying...", "err", err)
				nc.Close()
				time.Sleep(time.Second * 2)
				continue
			}

			return nc, js
		}

		log.Warn("Failed to connect to NATS, retrying in 2s...", "err", err)
		time.Sleep(time.Second * 2)
	}
}
