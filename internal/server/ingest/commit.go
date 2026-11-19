package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/log"

	"github.com/markojerkic/svarog/internal/server/logstore"
)

func (s *Service) commitBatch(ctx context.Context, batch []*pendingLog) error {
	if len(batch) == 0 {
		return nil
	}
	input := make([]logstore.Event, len(batch))
	for i, p := range batch {
		input[i] = p.input
	}
	delay := 250 * time.Millisecond
	retryClientDelete := true
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, dbWait)
		results, err := s.writer.Commit(attemptCtx, input)
		if err == nil {
			err = s.completeBatch(attemptCtx, batch, results)
			cancel()
			return err
		}
		cancel()
		log.Error("Error saving logs", "error", err)
		if s.live != nil && errors.Is(err, logstore.ErrUncertain) {
			scopes := make([]logstore.Scope, len(input))
			for i, event := range input {
				scopes[i] = event.Scope
			}
			s.live.AfterUncertainCommit(scopes)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, logstore.ErrRowData) {
			if len(batch) > 1 {
				mid := len(batch) / 2
				if err := s.commitBatch(ctx, batch[:mid]); err != nil {
					return err
				}
				return s.commitBatch(ctx, batch[mid:])
			}
			s.reject(batch[0], rejectData)
			return nil
		}
		if retryClientDelete && errors.Is(err, logstore.ErrClientDeleted) {
			// A committed delete can invalidate the statement snapshot. Retry once
			// with a fresh snapshot so the join can reject the missing client.
			retryClientDelete = false
		} else if errors.Is(err, logstore.ErrSystemic) || errors.Is(err, logstore.ErrClientDeleted) {
			return fmt.Errorf("systemic ingest write failure; deliveries left unacknowledged: %w", err)
		}
		log.Warn("Log commit failed; retrying without ACK", "error", err, "count", len(batch), "retry_in", delay)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		delay = min(delay*2, 5*time.Second)
	}
}

func (s *Service) completeBatch(ctx context.Context, batch []*pendingLog, results []logstore.Result) error {
	if len(results) != len(batch) {
		return fmt.Errorf("ingest writer returned %d outcomes for %d deliveries", len(results), len(batch))
	}

	var rows []logstore.Row
	for i, delivery := range batch {
		result := results[i]
		switch result.Outcome {
		case logstore.Inserted:
			s.finish(delivery, "acknowledge", delivery.message.Ack)
			if s.live != nil {
				rows = append(rows, result.Row)
			}
		case logstore.Duplicate:
			s.finish(delivery, "acknowledge", delivery.message.Ack)
		case logstore.RejectedClient:
			s.reject(delivery, rejectInactive)
		default:
			return fmt.Errorf("ingest writer returned unknown outcome %d for delivery %d", result.Outcome, i+1)
		}
	}
	if len(rows) > 0 {
		s.live.AfterCommit(ctx, rows)
	}
	return nil
}
