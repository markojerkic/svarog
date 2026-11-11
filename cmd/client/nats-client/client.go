package natsclient

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/cmd/client/config"
	"github.com/markojerkic/svarog/internal/lib/logevent"
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
		data, truncated, err := logevent.EncodePayload(*logLine)
		if err != nil {
			slog.Error("Failed to encode log line", "err", err)
			continue
		}
		if truncated {
			slog.Warn("Encoded log line exceeded the payload limit and was truncated", "limit", logevent.MaxPayloadBytes)
		}
		n.publish(data)
	}

	slog.Debug("Log lines channel closed, all messages published")
}

// jsErrMessageTooLarge is JetStream's stream max_msg_size rejection; a retry cannot succeed.
const jsErrMessageTooLarge jetstream.ErrorCode = 10054

// publish retries the exact encoded bytes, so the ID always identifies the stored event.
func (n *NatsClient) publish(data []byte) {
	delay := 250 * time.Millisecond
	// Reuse the ID so a lost ack does not duplicate the line within the stream's duplicate window.
	sum := logevent.MessageKey(n.config.Topic, data)
	id := hex.EncodeToString(sum[:])
	for {
		_, err := n.js.Publish(context.Background(), n.config.Topic, data, jetstream.WithMsgID(id))
		if err == nil {
			return
		}
		var apiErr *jetstream.APIError
		if errors.Is(err, nats.ErrMaxPayload) || (errors.As(err, &apiErr) && apiErr.ErrorCode == jsErrMessageTooLarge) {
			slog.Error("Could not publish log line; payload exceeds the server limit", "err", err)
			return
		}
		slog.Error("Failed to publish log line, retrying", "err", err, "retry_in", delay.String())
		time.Sleep(delay)
		delay = min(delay*2, 5*time.Second)
	}
}

func (n *NatsClient) Close() {
	if n.nc != nil {
		n.nc.Drain()
		n.nc.Close()
		slog.Debug("NATS connection closed")
	}
}

func (n *NatsClient) connectNats() {
	opts := []nats.Option{
		nats.UserCredentialBytes([]byte(n.config.Creds)),
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
