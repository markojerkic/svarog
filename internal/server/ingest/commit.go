package ingest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jackc/pgx/v5/pgconn"

	persistence "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
)

func (s *Service) commitBatch(ctx context.Context, batch []*pendingLog) error {
	if len(batch) == 0 {
		return nil
	}
	input := make([]db.IngestLog, len(batch))
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
		results, err := s.commitAndPublish(attemptCtx, input)
		cancel()
		if err == nil {
			if len(results) != len(batch) {
				return fmt.Errorf("ingest writer returned %d outcomes for %d deliveries", len(results), len(batch))
			}
			for i, p := range batch {
				result := results[i]
				if result.Ordinal != int64(i+1) {
					return fmt.Errorf("ingest writer returned ordinal %d for delivery %d", result.Ordinal, i+1)
				}
				switch result.Status {
				case persistence.IngestLogInserted, persistence.IngestLogDuplicate:
					s.finish(p, "acknowledge", p.message.Ack)
				case persistence.IngestLogRejectedClient:
					s.reject(p, rejectInactive)
				default:
					return fmt.Errorf("ingest writer returned unknown outcome %d for delivery %d", result.Status, i+1)
				}
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if rowDataError(err) {
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
		if retryClientDelete && persistence.IsConstraintViolation(err, persistence.SQLStateForeignKeyViolation, "logs_client_id_fkey") {
			// A committed delete can invalidate the statement snapshot. Retry once
			// with a fresh snapshot so the join can reject the missing client.
			retryClientDelete = false
		} else if systemicSaveError(err) {
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

func rowDataError(err error) bool {
	if errors.Is(err, db.ErrInvalidLogScope) {
		return true
	}
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) {
		return false
	}
	switch pgError.Code {
	case "22001", "22003", "22007", "22008", "22021", "22023", "22P02", "22P05":
		return true
	case "54000":
		// Not every program-limit error is row-specific (for example, stack
		// depth or an overly complex statement must not discard valid logs).
		return strings.Contains(pgError.Message, "index row") || strings.Contains(pgError.Message, "index entry") || strings.Contains(pgError.Message, "string is too long for tsvector")
	case "23514":
		return pgError.ConstraintName == "logs_sequence_number_check" || pgError.ConstraintName == "logs_event_key_check"
	default:
		return false
	}
}

func systemicSaveError(err error) bool {
	var pgError *pgconn.PgError
	if !errors.As(err, &pgError) || len(pgError.Code) < 2 {
		return false
	}
	switch pgError.Code[:2] {
	case "22", "23", "42", "28", "3D", "3F", "0A", "54", "XX":
		return true
	default:
		return false
	}
}

func (s *Service) commitAndPublish(ctx context.Context, input []db.IngestLog) ([]persistence.IngestLogResult, error) {
	results, err := s.writer.CommitLogs(ctx, input)
	if err != nil {
		log.Error("Error saving logs", "error", err)
		if s.live != nil && uncertainCommit(err) {
			rows := make([]types.StoredLog, len(input))
			for i, event := range input {
				rows[i] = event.Log
			}
			s.live.AfterUncertainCommit(rows)
		}
		return nil, err
	}
	if s.live != nil && len(results) == len(input) {
		rows := make([]types.StoredLog, 0, len(results))
		for i, result := range results {
			if result.Status == persistence.IngestLogInserted {
				row := input[i].Log
				row.ID = result.ID
				rows = append(rows, row)
			}
		}
		s.live.AfterCommit(ctx, rows)
	}
	return results, nil
}

// A backend error rolls the statement back. Connect and SafeToRetry errors
// happen before the statement reaches the server. Any other write failure can
// leave the outcome unknown.
func uncertainCommit(err error) bool {
	var pgError *pgconn.PgError
	var connectError *pgconn.ConnectError
	return !errors.Is(err, db.ErrInvalidLogScope) && !errors.As(err, &pgError) && !errors.As(err, &connectError) && !pgconn.SafeToRetry(err)
}
