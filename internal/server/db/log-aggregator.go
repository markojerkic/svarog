package db

import (
	"context"
	"errors"
	"sync"
	"time"

	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/markojerkic/svarog/internal/lib/backlog"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/types"
)

type LogLineWithHost struct {
	*rpc.LogLine
	ProjectId string
	ClientId  string
	Hostname  string
}

type AggregatingLogServer interface {
	Run(ctx context.Context, logIngestChannel <-chan LogLineWithHost)
	IsBacklogEmpty() bool
	BacklogCount() int
}

type LogServer struct {
	logService LogService
	backlog    backlog.Backlog[types.StoredLog]
}
type AvailableClient struct {
	Client   types.StoredClient
	IsOnline bool
}

var _ AggregatingLogServer = &LogServer{}

func NewLogServer(dbClient LogService) AggregatingLogServer {
	return &LogServer{
		logService: dbClient,
		backlog:    backlog.NewBacklog[types.StoredLog](1024 * 1024),
	}
}

const (
	logSaveRetryInitial = 250 * time.Millisecond
	logSaveRetryMax     = 5 * time.Second
)

// Already acknowledged, so retry a transient failure. Split a permanent one so a bad row cannot stall ingestion.
func (self *LogServer) saveBatch(ctx context.Context, logs []types.StoredLog) {
	err := self.saveWithRetry(ctx, logs)
	if err == nil {
		return
	}
	if ctx.Err() == nil && len(logs) > 1 {
		mid := len(logs) / 2
		self.saveBatch(ctx, logs[:mid])
		self.saveBatch(ctx, logs[mid:])
		return
	}

	message := "Could not save logs; error is not retryable"
	if ctx.Err() != nil {
		message = "Could not save logs; server is stopping"
	}
	args := []any{"error", err, "count", len(logs)}
	if len(logs) == 1 {
		args = append(args, "client_id", logs[0].Client.ClientId, "sequence", logs[0].SequenceNumber)
	}
	slog.Error(message, args...)
}

func (self *LogServer) saveWithRetry(ctx context.Context, logs []types.StoredLog) error {
	delay := logSaveRetryInitial
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := self.logService.SaveLogs(ctx, logs)
		if err == nil {
			if attempt > 1 {
				slog.Info("Saved logs after retry", "count", len(logs), "attempts", attempt)
			}
			return nil
		}
		if ctx.Err() != nil || !retryableSaveError(err) {
			return err
		}

		slog.Error("Could not save logs, retrying", "error", err, "count", len(logs), "attempt", attempt, "retry_in", delay.String())
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		delay = min(delay*2, logSaveRetryMax)
	}
}

func retryableSaveError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, errInvalidLogClientID) {
		return false
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && len(pgError.Code) >= 2 {
		switch pgError.Code[:2] {
		case "22", "23", "42":
			return false
		}
	}
	return true
}

func (self *LogServer) Run(ctx context.Context, logIngestChannel <-chan LogLineWithHost) {
	slog.Debug("Starting log server")
	interval := time.NewTicker(2 * time.Second)
	defer interval.Stop()

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for logsToSave := range self.backlog.GetLogs() {
			self.saveBatch(ctx, logsToSave)
		}
	}()

outer:
	for {
		select {
		case line, ok := <-logIngestChannel:
			if !ok {
				self.backlog.ForceDump()
				self.backlog.Close()
				break outer
			}
			logLine := types.StoredLog{
				LogLine:        line.Message,
				Timestamp:      line.Timestamp,
				SequenceNumber: line.Sequence,
				Client: types.StoredClient{
					ProjectId:  line.ProjectId,
					ClientId:   line.ClientId,
					InstanceId: line.Hostname,
				},
			}
			self.backlog.AddToBacklog(logLine)

		case <-interval.C:
			self.backlog.ForceDump()

		case <-ctx.Done():
			slog.Debug("Context done")
			self.backlog.Close()
			break outer
		}
	}
}

func (self *LogServer) IsBacklogEmpty() bool {
	return self.backlog.IsEmpty()
}

func (self *LogServer) BacklogCount() int {
	return self.backlog.Count()
}
