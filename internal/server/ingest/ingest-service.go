package ingest

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/nats-io/nats.go/jetstream"
)

type IngestService struct {
	ingestCh   chan db.LogLineWithHost
	natsConn   *natsconn.NatsConnection
	consumeCtx jetstream.ConsumeContext
	queries    *sqlc.Queries
}

func NewIngestService(ingestCh chan db.LogLineWithHost, natsConn *natsconn.NatsConnection, pool *pgxpool.Pool) *IngestService {
	return &IngestService{
		ingestCh: ingestCh,
		natsConn: natsConn,
		queries:  sqlc.New(pool),
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
		parts := strings.Split(msg.Subject(), ".")
		if len(parts) != 3 || parts[0] != "logs" {
			msg.Term()
			return
		}
		projectID, projectErr := uuid.Parse(parts[1])
		clientID, clientErr := uuid.Parse(parts[2])
		if projectErr != nil || clientErr != nil || projectID.String() != parts[1] || clientID.String() != parts[2] {
			msg.Term()
			return
		}
		var logLine rpc.LogLine
		if err := json.Unmarshal(msg.Data(), &logLine); err != nil {
			slog.Error("Failed to unmarshal log line", "err", err)
			msg.Term()
			return
		}

		if err := logLine.Validate(); err != nil {
			slog.Error("Invalid log line", "err", err)
			msg.Term()
			return
		}

		queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		active, err := i.queries.ActiveProjectClientExists(queryCtx, sqlc.ActiveProjectClientExistsParams{
			ProjectID: projectID, ID: clientID,
		})
		cancel()
		if err != nil {
			slog.Error("Failed to validate log client", "err", err)
			msg.NakWithDelay(time.Second)
			return
		}
		if !active {
			msg.Term()
			return
		}

		select {
		case i.ingestCh <- db.LogLineWithHost{
			LogLine:   &logLine,
			ClientId:  clientID.String(),
			ProjectId: projectID.String(),
			Hostname:  logLine.InstanceId,
		}:
			msg.Ack()
		case <-ctx.Done():
			msg.Nak()
		}
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
