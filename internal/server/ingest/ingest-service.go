package ingest

import (
	"context"
	"encoding/json"
	"strings"

	"log/slog"

	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/nats-io/nats.go/jetstream"
)

type IngestService struct {
	ingestCh   chan db.LogLineWithHost
	natsConn   *natsconn.NatsConnection
	consumeCtx jetstream.ConsumeContext
}

func NewIngestService(ingestCh chan db.LogLineWithHost, natsConn *natsconn.NatsConnection) *IngestService {
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

	consumeCtx, err := consumer.Consume(func(msg jetstream.Msg) {
		var logLine rpc.LogLine
		if err := json.Unmarshal(msg.Data(), &logLine); err != nil {
			slog.Error("Failed to unmarshal log line", "err", err)
			msg.Nak()
			return
		}

		if err := logLine.Validate(); err != nil {
			slog.Error("Invalid log line", "err", err)
			msg.Term()
			return
		}

		subject := msg.Subject()
		parts := strings.Split(subject, ".")
		projectId := parts[len(parts)-2]
		clientId := parts[len(parts)-1]

		slog.Debug("Received log line", "projectId", projectId, "clientId", clientId, "subject", subject)
		i.ingestCh <- db.LogLineWithHost{
			LogLine:   &logLine,
			ClientId:  clientId,
			ProjectId: projectId,
			Hostname:  "<TODO>",
		}
		msg.Ack()
	}, jetstream.PullMaxMessages(100))

	if err != nil {
		return err
	}

	i.consumeCtx = consumeCtx

	// Block until context is cancelled
	<-ctx.Done()
	slog.Info("Ingest service shutting down")
	return nil
}

func (i *IngestService) Stop() {
	if i.consumeCtx != nil {
		i.consumeCtx.Stop()
	}
}
