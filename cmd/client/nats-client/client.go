package natsclient

import (
	"context"
	"encoding/json"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/cmd/client/config"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NatsClient struct {
	config   config.ClientConfig
	nc       *nats.Conn
	js       jetstream.JetStream
	logLines <-chan *rpc.LogLine
}

func NewNatsClient(cfg config.ClientConfig, logLines <-chan *rpc.LogLine) *NatsClient {
	return &NatsClient{
		config:   cfg,
		logLines: logLines,
	}
}

func (n *NatsClient) Run() {
	defer n.Close()

	n.connectNats()

	for logLine := range n.logLines {
		data, err := json.Marshal(logLine)
		if err != nil {
			slog.Error("Failed to marshal log line", "err", err)
			continue
		}

		if _, err := n.js.Publish(context.Background(), n.config.Topic, data); err != nil {
			slog.Error("Failed to publish log line", "err", err)
		}
	}

	slog.Debug("Log lines channel closed, all messages published")
}

func (n *NatsClient) Close() {
	if n.nc != nil {
		n.nc.Drain()
		n.nc.Close()
		slog.Debug("NATS connection closed")
	}
}

func (n *NatsClient) connectNats() {
	// Parse credentials from the creds file content
	jwt, seed, err := serverauth.ParseCredsFile(n.config.Creds)
	if err != nil {
		slog.Error("Failed to parse credentials", "err", err)
		panic(err)
	}

	opts := []nats.Option{
		nats.UserJWTAndSeed(jwt, seed),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				slog.Error("Disconnected from NATS", "err", err)
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Debug("Reconnected to NATS")
		}),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			slog.Error("NATS error", "err", err)
		}),
	}

	retryDelay := time.Second * 2

	for {
		nc, err := nats.Connect(n.config.GetNatsUrl(), opts...)
		if err == nil {
			slog.Debug("Connected to NATS", "url", n.config.GetNatsUrl())

			js, err := jetstream.New(nc)
			if err != nil {
				slog.Error("Failed to create JetStream context, retrying...", "err", err)
				nc.Close()
				time.Sleep(retryDelay)
				continue
			}

			n.nc = nc
			n.js = js
			return
		}

		slog.Warn("Failed to connect to NATS, retrying in 2s...", "err", err)
		time.Sleep(retryDelay)
	}
}
