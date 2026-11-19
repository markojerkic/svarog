package logstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

var (
	ErrRowData       = errors.New("invalid log row data")
	ErrUncertain     = errors.New("log commit outcome is uncertain")
	ErrClientDeleted = errors.New("log client was deleted during commit")
	ErrSystemic      = errors.New("systemic log commit failure")
)

// Commit checks client ownership and activity in the insert statement. A
// successful call returns one outcome per event in input order. Event keys
// are retained only as long as their logs are retained. Known transient
// failures are returned with their original cause for ordinary retries.
func (s *Store) Commit(ctx context.Context, events []Event) ([]Result, error) {
	if len(events) == 0 {
		return nil, nil
	}
	params := sqlc.InsertIngestLogsParams{
		ClientIds: make([]uuid.UUID, len(events)), ProjectIds: make([]uuid.UUID, len(events)),
		InstanceNames: make([]string, len(events)), OccurredAt: make([]time.Time, len(events)),
		SequenceNumbers: make([]int64, len(events)), Messages: make([]string, len(events)),
		EventKeys: make([][]byte, len(events)),
	}
	for i := range events {
		event := &events[i]
		params.ClientIds[i] = event.Scope.ClientID
		params.ProjectIds[i] = event.Scope.ProjectID
		params.InstanceNames[i] = event.Instance
		params.OccurredAt[i] = event.OccurredAt
		params.SequenceNumbers[i] = event.Sequence
		params.Messages[i] = event.Message
		params.EventKeys[i] = event.Key[:]
	}
	rows, err := s.queries.InsertIngestLogs(ctx, params)
	if err != nil {
		return nil, classifyCommitError(fmt.Errorf("insert ingest logs: %w", err))
	}
	if len(rows) != len(events) {
		return nil, fmt.Errorf("%w: got %d results for %d events", ErrSystemic, len(rows), len(events))
	}
	results := make([]Result, len(rows))
	for i, row := range rows {
		if row.Ordinal != int64(i+1) {
			return nil, fmt.Errorf("%w: got ordinal %d for event %d", ErrSystemic, row.Ordinal, i+1)
		}
		switch {
		case !row.ActiveClient:
			results[i].Outcome = RejectedClient
		case row.ID == 0:
			results[i].Outcome = Duplicate
		default:
			event := &events[i]
			results[i] = Result{
				Outcome: Inserted,
				Row: Row{ID: row.ID, Scope: event.Scope, Instance: event.Instance,
					OccurredAt: event.OccurredAt, Sequence: event.Sequence, Message: event.Message},
			}
		}
	}
	return results, nil
}

func classifyCommitError(err error) error {
	var pgError *pgconn.PgError
	var connectError *pgconn.ConnectError
	if errors.As(err, &pgError) {
		// A backend error rolls the statement back. Only known row-specific
		// failures can safely be isolated by retrying smaller batches.
		switch {
		case rowDataError(pgError):
			return fmt.Errorf("%w: %w", ErrRowData, err)
		case pgError.Code == "23503" && pgError.ConstraintName == "logs_client_id_fkey":
			return fmt.Errorf("%w: %w", ErrClientDeleted, err)
		case systemicError(pgError.Code):
			return fmt.Errorf("%w: %w", ErrSystemic, err)
		default:
			return err
		}
	}
	// Connection setup and safe-to-retry errors occurred before the statement
	// reached PostgreSQL. Other failures may have lost a committed result.
	if errors.As(err, &connectError) || pgconn.SafeToRetry(err) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrUncertain, err)
}

func rowDataError(err *pgconn.PgError) bool {
	switch err.Code {
	case "22001", "22003", "22007", "22008", "22021", "22023", "22P02", "22P05":
		return true
	case "54000":
		return strings.Contains(err.Message, "index row") || strings.Contains(err.Message, "index entry") || strings.Contains(err.Message, "string is too long for tsvector")
	case "23514":
		return err.ConstraintName == "logs_sequence_number_check" || err.ConstraintName == "logs_event_key_check"
	default:
		return false
	}
}

func systemicError(code string) bool {
	if len(code) < 2 {
		return false
	}
	switch code[:2] {
	case "22", "23", "42", "28", "3D", "3F", "0A", "54", "XX":
		return true
	default:
		return false
	}
}
