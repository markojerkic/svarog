package ingest

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/server/db"
)

type LogWriter interface {
	// A nil error means every event is committed or its key already exists in
	// logs. Writers must honor ctx; shutdown waits for their active call to exit.
	CommitLogs(ctx context.Context, logs []db.IngestLog) error
}

type Options struct {
	Connection              *natsconn.NatsConnection
	Pool                    *pgxpool.Pool
	Writer                  LogWriter
	StreamMaxBytes          int64 // Zero preserves an existing finite limit; new streams default to 1 GiB.
	AckWait                 time.Duration
	AllowRetentionMigration bool // Only after backup/preflight and stopping all old writers.
}

const (
	streamName      = "LOGS"
	consumerName    = "log-processor"
	logSubject      = "logs.>"
	maxMessageBytes = 1 << 20
	maxBatchBytes   = 4 << 20
	maxBatchRows    = 1000
	fetchRows       = 32
	fetchWait       = 250 * time.Millisecond
	batchWait       = 2 * time.Second
	dbWait          = 10 * time.Second
)
