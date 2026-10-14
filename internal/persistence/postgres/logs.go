package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

func InsertLogs(ctx context.Context, pool *pgxpool.Pool, logs []LogInsert) ([]int64, error) {
	if len(logs) == 0 {
		return nil, nil
	}
	arg := sqlc.InsertLogsParams{
		ClientIds:       make([]uuid.UUID, len(logs)),
		InstanceNames:   make([]string, len(logs)),
		OccurredAt:      make([]time.Time, len(logs)),
		SequenceNumbers: make([]int64, len(logs)),
		Messages:        make([]string, len(logs)),
	}
	for i, log := range logs {
		arg.ClientIds[i] = log.ClientID
		arg.InstanceNames[i] = log.InstanceName
		arg.OccurredAt[i] = log.OccurredAt
		arg.SequenceNumbers[i] = log.SequenceNumber
		arg.Messages[i] = log.Message
	}

	ids, err := sqlc.New(pool).InsertLogs(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("insert logs: %w", err)
	}
	if len(ids) != len(logs) {
		return nil, fmt.Errorf("insert logs: got %d ids for %d rows", len(ids), len(logs))
	}
	return ids, nil
}
