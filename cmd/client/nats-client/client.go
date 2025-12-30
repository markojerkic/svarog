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
	nc       *nats.Conn
	js       jetstream.JetStream
	logLines <-chan *commontypes.LogLineDto
}

func NewNatsClient(cfg config.ClientConfig, logLines <-chan *commontypes.LogLineDto) *NatsClient {
	return &NatsClient{
		config:   cfg,
		logLines: logLines,
	}
}

func (n *NatsClient) Run(ctx context.Context) {
	defer n.Close()

	nc, js := n.connectNats(ctx)
	if nc == nil {
		log.Debug("NATS connection cancelled")
		return
	}
	n.nc = nc
	n.js = js

	for {
		select {
		case <-ctx.Done():
			log.Debug("NATS client context done")
			return
		case logLine, ok := <-n.logLines:
			if !ok {
				log.Debug("Log lines channel closed, exiting")
				return
			}

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

func (n *NatsClient) Close() {
	if n.nc != nil {
		n.nc.Drain()
		n.nc.Close()
		log.Debug("NATS connection closed")
	}
}

func (n *NatsClient) connectNats(ctx context.Context) (*nats.Conn, jetstream.JetStream) {
	opts := []nats.Option{
		nats.Token(n.config.Token),
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

	retryDelay := time.Second * 2

	for {
		select {
		case <-ctx.Done():
			return nil, nil
		default:
		}

		nc, err := nats.Connect(n.config.GetNatsUrl(), opts...)
		if err == nil {
			log.Debug("Connected to NATS", "url", n.config.GetNatsUrl())

			js, err := jetstream.New(nc)
			if err != nil {
				log.Error("Failed to create JetStream context, retrying...", "err", err)
				nc.Close()
				select {
				case <-ctx.Done():
					return nil, nil
				case <-time.After(retryDelay):
				}
				continue
			}

			return nc, js
		}

		log.Warn("Failed to connect to NATS, retrying in 2s...", "err", err)
		select {
		case <-ctx.Done():
			return nil, nil
		case <-time.After(retryDelay):
		}
	}
}
