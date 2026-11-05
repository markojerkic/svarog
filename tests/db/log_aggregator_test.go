package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

func TestAggregatorSaveFailure(t *testing.T) {
	t.Run("retries until the batch is stored", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		svc := &flakyLogService{
			failFor: 2,
			err:     &pgconn.PgError{Code: "40001", Message: "could not serialize access"},
		}

		runAggregator(t, ctx, svc, []db.LogLineWithHost{aggregatorLine("kept", 7)})

		require.Equal(t, 3, svc.attempts, "a serialization failure is retried until the same batch is stored")
		saved := svc.saved
		require.Len(t, saved, 1, "the failed batch is saved once, not dropped or duplicated")
		require.Equal(t, "kept", saved[0].LogLine, "retry persists the message from the failed attempt")
		require.Equal(t, 7, saved[0].SequenceNumber, "retry persists the sequence from the failed attempt")
		require.Equal(t, "web", saved[0].Client.InstanceId, "retry persists the instance from the failed attempt")
	})

	t.Run("does not retry a permanent error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		svc := &flakyLogService{
			failFor: 10,
			err:     &pgconn.PgError{Code: "23503", Message: "logs_client_id_fkey"},
		}

		runAggregator(t, ctx, svc, []db.LogLineWithHost{aggregatorLine("orphan", 1)})

		require.Equal(t, 1, svc.attempts, "a foreign key violation cannot succeed later and must not stall later batches")
		require.Empty(t, svc.saved, "a batch that cannot be stored is not reported as saved")
	})

	t.Run("keeps the rest of a batch when one row cannot be stored", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		svc := &flakyLogService{
			rejectMessage: "bad",
			err:           &pgconn.PgError{Code: "23503", Message: "logs_client_id_fkey"},
		}

		runAggregator(t, ctx, svc, []db.LogLineWithHost{
			aggregatorLine("left", 1),
			aggregatorLine("bad", 2),
			aggregatorLine("right", 3),
		})

		var messages []string
		for _, log := range svc.saved {
			messages = append(messages, log.LogLine)
		}
		require.Equal(t, []string{"left", "right"}, messages, "a permanent failure must not drop the other rows in the same insert")
	})

	t.Run("stops when the server is stopping", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		svc := &flakyLogService{started: make(chan struct{})}
		server := db.NewLogServer(svc)
		ch := make(chan db.LogLineWithHost)
		done := make(chan struct{})
		go func() {
			defer close(done)
			server.Run(ctx, ch)
		}()

		for i := range 2000 {
			ch <- aggregatorLine("in flight", i)
		}
		close(ch)
		select {
		case <-svc.started:
		case <-time.After(2 * time.Second):
			t.Fatal("save must start so cancellation can interrupt the in-flight batch")
		}

		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("stopping the server must end the save retry instead of leaving it running")
		}
		require.Equal(t, 1, svc.attempts, "cancellation stops retries and prevents saving queued batches")
		require.Empty(t, svc.saved, "a save interrupted by shutdown is not reported as stored")
	})
}

func runAggregator(t *testing.T, ctx context.Context, svc db.LogService, lines []db.LogLineWithHost) {
	t.Helper()
	server := db.NewLogServer(svc)
	ch := make(chan db.LogLineWithHost)
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.Run(ctx, ch)
	}()
	for _, line := range lines {
		select {
		case ch <- line:
		case <-ctx.Done():
			t.Fatal("aggregator must accept the line before the deadline")
		}
	}
	close(ch)
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("aggregator must finish the batch before the deadline")
	}
}

func aggregatorLine(message string, sequence int) db.LogLineWithHost {
	return db.LogLineWithHost{
		LogLine: &rpc.LogLine{
			Message:   message,
			Timestamp: time.Unix(1_700_000_000, 0).UTC(),
			Sequence:  sequence,
		},
		ProjectId: "project",
		ClientId:  "client",
		Hostname:  "web",
	}
}

type flakyLogService struct {
	db.LogService
	attempts      int
	failFor       int
	err           error
	rejectMessage string
	saved         []types.StoredLog
	started       chan struct{}
}

func (self *flakyLogService) SaveLogs(ctx context.Context, logs []types.StoredLog) error {
	self.attempts++
	if self.started != nil {
		if self.attempts == 1 {
			close(self.started)
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if self.rejectMessage != "" {
		for _, log := range logs {
			if log.LogLine == self.rejectMessage {
				return fmt.Errorf("insert logs: %w", self.err)
			}
		}
	}
	if self.attempts <= self.failFor {
		return fmt.Errorf("insert logs: %w", self.err)
	}

	self.saved = append(self.saved, logs...)
	return nil
}
