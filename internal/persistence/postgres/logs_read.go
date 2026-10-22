package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

func ListLogs(ctx context.Context, pool *pgxpool.Pool, query LogReadQuery) ([]LogRow, error) {
	instances := query.Instances
	if instances == nil {
		instances = []string{}
	}
	var cursorID int64
	hasCursorID := query.CursorID != nil
	if hasCursorID {
		cursorID = *query.CursorID
	}
	rows, err := sqlc.New(pool).ListLogs(ctx, sqlc.ListLogsParams{
		ProjectID:       query.ProjectID,
		ClientID:        query.ClientID,
		FilterInstances: query.FilterInstances,
		Instances:       instances,
		HasCursor:       query.HasCursor,
		Backward:        query.Backward,
		Inclusive:       query.Inclusive,
		CursorAt:        query.CursorAt,
		CursorSequence:  query.CursorSequence,
		CursorID:        cursorID,
		HasCursorID:     hasCursorID,
		PageSize:        query.PageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("list logs: %w", err)
	}
	if query.HasCursor && !query.Backward {
		slices.Reverse(rows)
	}
	logs := make([]LogRow, len(rows))
	for i, row := range rows {
		logs[i] = LogRow{
			ID:             row.ID,
			ClientID:       row.ClientID,
			ProjectID:      row.ProjectID,
			InstanceName:   row.InstanceName,
			OccurredAt:     row.OccurredAt,
			SequenceNumber: row.SequenceNumber,
			Message:        row.Message,
		}
	}
	return logs, nil
}

func SearchLogs(ctx context.Context, pool *pgxpool.Pool, query LogSearchQuery) ([]LogRow, error) {
	instances := query.Instances
	if instances == nil {
		instances = []string{}
	}
	var cursorID int64
	hasCursorID := query.CursorID != nil
	if hasCursorID {
		cursorID = *query.CursorID
	}
	rows, err := sqlc.New(pool).SearchLogs(ctx, sqlc.SearchLogsParams{
		ProjectID:       query.ProjectID,
		ClientID:        query.ClientID,
		Search:          query.Search,
		FilterInstances: query.FilterInstances,
		Instances:       instances,
		HasCursor:       query.HasCursor,
		Backward:        query.Backward,
		Inclusive:       query.Inclusive,
		HasCursorID:     hasCursorID,
		CursorAt:        query.CursorAt,
		CursorSequence:  query.CursorSequence,
		CursorID:        cursorID,
		PageSize:        query.PageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("search logs: %w", err)
	}
	if query.HasCursor && !query.Backward {
		slices.Reverse(rows)
	}
	logs := make([]LogRow, len(rows))
	for i, row := range rows {
		logs[i] = LogRow{
			ID:             row.ID,
			ClientID:       row.ClientID,
			ProjectID:      row.ProjectID,
			InstanceName:   row.InstanceName,
			OccurredAt:     row.OccurredAt,
			SequenceNumber: row.SequenceNumber,
			Message:        row.Message,
		}
	}
	return logs, nil
}

func GetLogAnchor(ctx context.Context, pool *pgxpool.Pool, projectID, clientID uuid.UUID, id int64) (LogRow, error) {
	row, err := sqlc.New(pool).GetLogAnchor(ctx, sqlc.GetLogAnchorParams{
		ID:        id,
		ClientID:  clientID,
		ProjectID: projectID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LogRow{}, err
		}
		return LogRow{}, fmt.Errorf("get log anchor: %w", err)
	}
	return LogRow{
		ID:             row.ID,
		ClientID:       row.ClientID,
		ProjectID:      row.ProjectID,
		InstanceName:   row.InstanceName,
		OccurredAt:     row.OccurredAt,
		SequenceNumber: row.SequenceNumber,
		Message:        row.Message,
	}, nil
}

func ListInstances(ctx context.Context, pool *pgxpool.Pool, projectID, clientID uuid.UUID) ([]string, error) {
	instances, err := sqlc.New(pool).ListInstances(ctx, sqlc.ListInstancesParams{
		ProjectID: projectID,
		ClientID:  clientID,
	})
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	if instances == nil {
		return []string{}, nil
	}
	return instances, nil
}
