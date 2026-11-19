package logstore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

func (s *Store) Page(ctx context.Context, query PageQuery) (Page, error) {
	if query.Scope.ProjectID == uuid.Nil || query.Scope.ClientID == uuid.Nil {
		return Page{}, nil
	}
	if err := validateLimit(query.Limit); err != nil {
		return Page{}, err
	}
	if query.Older != nil && query.Newer != nil {
		return Page{}, errors.New("log page requires at most one cursor")
	}
	if (query.Older != nil && query.Older.ID <= 0) || (query.Newer != nil && query.Newer.ID <= 0) {
		return Page{}, errors.New("log cursor requires a positive row ID")
	}

	var page Page
	if query.AnchorID > 0 && query.Search == nil {
		anchor, found, err := s.anchor(ctx, query.Scope, query.AnchorID)
		if err != nil {
			return Page{}, err
		}
		if found {
			query.Instances = []string{anchor.Instance}
			page.AnchorInstance = &anchor.Instance
		}
	}

	rows, err := s.list(ctx, query)
	if err != nil {
		return Page{}, err
	}
	page.Rows = rows
	if len(rows) == query.Limit {
		if query.Newer != nil {
			page.Newer = cursorFor(rows[0])
		} else {
			page.Older = cursorFor(rows[len(rows)-1])
		}
	}
	return page, nil
}

// Around returns the authorized anchor with up to limit neighbors on each side.
func (s *Store) Around(ctx context.Context, scope Scope, id int64, limit int, bounds *TimeRange) (Page, bool, error) {
	if id <= 0 || scope.ProjectID == uuid.Nil || scope.ClientID == uuid.Nil {
		return Page{}, false, nil
	}
	if err := validateLimit(limit); err != nil {
		return Page{}, false, err
	}
	anchor, found, err := s.anchor(ctx, scope, id)
	if err != nil || !found {
		return Page{}, found, err
	}
	query := PageQuery{
		Scope: scope, Instances: []string{anchor.Instance}, Range: bounds,
		Older: cursorFor(anchor), Limit: limit,
	}
	older, err := s.Page(ctx, query)
	if err != nil {
		return Page{}, false, err
	}
	query.Older, query.Newer = nil, query.Older
	newer, err := s.Page(ctx, query)
	if err != nil {
		return Page{}, false, err
	}
	return Page{
		Rows:  append(append(newer.Rows, anchor), older.Rows...),
		Older: older.Older, Newer: newer.Newer, AnchorInstance: &anchor.Instance,
	}, true, nil
}

func (s *Store) Instances(ctx context.Context, scope Scope) ([]string, error) {
	if scope.ProjectID == uuid.Nil || scope.ClientID == uuid.Nil {
		return []string{}, nil
	}
	instances, err := s.queries.ListInstances(ctx, sqlc.ListInstancesParams{
		ProjectID: scope.ProjectID, ClientID: scope.ClientID,
	})
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	if instances == nil {
		return []string{}, nil
	}
	return instances, nil
}

func validateLimit(limit int) error {
	if limit <= 0 || limit > math.MaxInt32 {
		return fmt.Errorf("invalid log page size %d", limit)
	}
	return nil
}

func cursorFor(row Row) *Cursor {
	return &Cursor{OccurredAt: row.OccurredAt, Sequence: row.Sequence, ID: row.ID}
}

func (s *Store) anchor(ctx context.Context, scope Scope, id int64) (Row, bool, error) {
	row, err := s.queries.GetLogAnchor(ctx, sqlc.GetLogAnchorParams{
		ID: id, ClientID: scope.ClientID, ProjectID: scope.ProjectID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Row{}, false, nil
	}
	if err != nil {
		return Row{}, false, fmt.Errorf("get log anchor: %w", err)
	}
	return readRow(sqlc.ListLogsRow(row)), true, nil
}

func (s *Store) list(ctx context.Context, query PageQuery) ([]Row, error) {
	params := sqlc.ListLogsParams{
		ProjectID: query.Scope.ProjectID, ClientID: query.Scope.ClientID,
		FilterInstances: query.Instances != nil,
		Instances:       query.Instances, PageSize: int32(query.Limit),
	}
	if params.Instances == nil {
		params.Instances = []string{}
	}
	if query.Range != nil {
		params.HasRange = true
		params.RangeFrom, params.RangeTo = query.Range.From, query.Range.To
	}
	cursor := query.Newer
	if query.Older != nil {
		cursor = query.Older
		params.Backward = true
	}
	if cursor != nil {
		params.HasCursor = true
		params.CursorAt, params.CursorSequence, params.CursorID = cursor.OccurredAt, cursor.Sequence, cursor.ID
	}
	var rows []Row
	if query.Search == nil {
		found, err := s.queries.ListLogs(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("list logs: %w", err)
		}
		rows = make([]Row, len(found))
		for i, row := range found {
			rows[i] = readRow(row)
		}
	} else {
		if len(query.Search.Terms) == 0 {
			return nil, nil
		}
		found, err := s.search.SearchLogs(ctx, sqlc.SearchLogsParams{
			ProjectID: params.ProjectID, ClientID: params.ClientID,
			PrimaryPattern: query.Search.PrimaryPattern, RemainingPatterns: query.Search.RemainingPatterns,
			FilterInstances: params.FilterInstances, Instances: params.Instances,
			HasRange: params.HasRange, RangeFrom: params.RangeFrom, RangeTo: params.RangeTo,
			HasCursor: params.HasCursor, Backward: params.Backward,
			CursorAt: params.CursorAt, CursorSequence: params.CursorSequence, CursorID: params.CursorID,
			PageSize: params.PageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("search logs: %w", err)
		}
		rows = make([]Row, len(found))
		for i, row := range found {
			rows[i] = readRow(sqlc.ListLogsRow(row))
		}
	}
	if query.Newer != nil {
		slices.Reverse(rows)
	}
	return rows, nil
}

func readRow(row sqlc.ListLogsRow) Row {
	return Row{
		ID: row.ID, Scope: Scope{ProjectID: row.ProjectID, ClientID: row.ClientID},
		Instance: row.InstanceName, OccurredAt: row.OccurredAt, Sequence: row.SequenceNumber, Message: row.Message,
	}
}

// Search selectivity and cursor direction vary per request. Cache parameter
// descriptions, but plan with the actual values instead of using a generic plan.
type searchQueryDB struct {
	sqlc.DBTX
}

func (db searchQueryDB) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return db.DBTX.Query(ctx, query, append([]any{pgx.QueryExecModeCacheDescribe}, args...)...)
}
