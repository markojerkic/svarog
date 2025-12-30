package natsclient

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/cmd/client/config"
	"github.com/markojerkic/svarog/internal/commontypes"
	"github.com/nats-io/nats.go"
)

type NatsClient struct {
	config   config.ClientConfig
	conn     *nats.Conn
	logLines chan *commontypes.LogLineDto
}

func NewNatsClient(config config.ClientConfig, logLines chan *commontypes.LogLineDto) NatsClient {
	nc, err := connectNats(config)
	if err != nil {
		log.Fatal("Failed to connect to NATS", "err", err)
	}

	return NatsClient{
		config:   config,
		conn:     nc,
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
			if err := n.conn.Publish(n.config.Topic, []byte(logLine)); err != nil {
				log.Error("Failed to publish log line", "err", err)
			}
		}
	}

}

func connectNats(config config.ClientConfig) (*nats.Conn, error) {
	return nats.Connect(config.GetNatsUrl(),
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

}
