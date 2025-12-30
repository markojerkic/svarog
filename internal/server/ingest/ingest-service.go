package ingest

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/nats-io/nats.go/jetstream"
)

type IngestService struct {
	ingestCh chan db.LogLineWithHost
	natsConn *serverauth.NatsConnection
}

func NewIngestService(ingestCh chan db.LogLineWithHost, natsConn *serverauth.NatsConnection) *IngestService {
	return &IngestService{
		ingestCh: ingestCh,
		natsConn: natsConn,
	}
}

func (i *IngestService) Run(ctx context.Context) error {
	consumer, err := i.natsConn.JetStream.CreateOrUpdateConsumer(ctx, "LOGS", jetstream.ConsumerConfig{
		Durable:       "log-processor",
		FilterSubject: "logs.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})

	if err != nil {
		return err
	}

	_, err = consumer.Consume(func(msg jetstream.Msg) {
		var logLine rpc.LogLine
		if err := json.Unmarshal(msg.Data(), &logLine); err != nil {
			log.Error("Failed to unmarshal log line", "err", err)
			return
		}
		msg.Ack()
		subject := msg.Subject()
		parts := strings.Split(subject, ".")
		clientId := parts[len(parts)-1]

		log.Debug("Received log line", "clientId", clientId, "subject", subject)
		i.ingestCh <- db.LogLineWithHost{
			LogLine:  &logLine,
			ClientId: clientId,
			Hostname: "<TODO>",
		}

	})

	return err
}
