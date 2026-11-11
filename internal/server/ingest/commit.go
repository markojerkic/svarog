package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/markojerkic/svarog/internal/server/db"
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
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, dbWait)
		err := s.writer.CommitLogs(attemptCtx, input)
		cancel()
		if err == nil {
			for _, p := range batch {
				s.finish(p, "acknowledge", p.message.Ack)
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var pgError *pgconn.PgError
		foreign := errors.As(err, &pgError) && pgError.Code == "23503" && pgError.ConstraintName == "logs_client_id_fkey"
		if foreign || rowDataError(err) {
			if len(batch) > 1 {
				mid := len(batch) / 2
				if err := s.commitBatch(ctx, batch[:mid]); err != nil {
					return err
				}
				return s.commitBatch(ctx, batch[mid:])
			}
			if !foreign {
				s.reject(batch[0], rejectData)
				return nil
			}
			log := batch[0].input.Log
			active, lookupErr := s.active(ctx, uuid.MustParse(log.Client.ProjectId), uuid.MustParse(log.Client.ClientId))
			if lookupErr == nil {
				if active {
					return fmt.Errorf("unexpected active-client log foreign key failure: %w", err)
				}
				s.reject(batch[0], rejectInactive)
				return nil
			}
		} else if systemicSaveError(err) {
			return fmt.Errorf("systemic ingest write failure; deliveries left unacknowledged: %w", err)
		}
		slog.Warn("Log commit failed; retrying without ACK", "error", err, "count", len(batch), "retry_in", delay)
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
	if errors.Is(err, db.ErrInvalidLogClientID) {
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
