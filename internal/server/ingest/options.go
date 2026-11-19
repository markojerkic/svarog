package ingest

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/server/logstore"
)

type Writer interface {
	// On success, return one result per event in input order. A result describes
	// an inserted event, a duplicate, or a rejected client. Writers must honor
	// ctx; shutdown waits for their active call to exit.
	Commit(ctx context.Context, logs []logstore.Event) ([]logstore.Result, error)
}

type Live interface {
	AfterCommit(ctx context.Context, rows []logstore.Row)
	AfterUncertainCommit(scopes []logstore.Scope)
}

type Options struct {
	Connection              *natsconn.NatsConnection
	Pool                    *pgxpool.Pool
	Writer                  Writer
	Live                    Live
	StreamMaxBytes          int64 // Zero preserves an existing finite limit; new streams default to 1 GiB.
	AckWait                 time.Duration
	AllowRetentionMigration bool // Only after backup/preflight and stopping all old writers.
}

const (
	streamName      = "LOGS"
	consumerName    = "log-processor"
	logSubject      = "logs.>"
	maxMessageBytes = logevent.MaxMessageBytes
	maxBatchBytes   = 4 << 20
	maxBatchRows    = 1000
	fetchRows       = 32
	fetchWait       = 250 * time.Millisecond
	// A partial batch is committed instead of issuing a pull shorter than this.
	minFetchWait  = 50 * time.Millisecond
	batchWait     = 2 * time.Second
	dbWait        = 10 * time.Second
	fetchRetryMin = 250 * time.Millisecond
	fetchRetryMax = 5 * time.Second
	// Every this many consecutive pull failures, confirm that the durable still exists.
	inspectFetchFailures = 3
)
