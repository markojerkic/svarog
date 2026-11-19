package logstore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	return readRow(logReadRow(row)), true, nil
}

func (s *Store) list(ctx context.Context, query PageQuery) ([]Row, error) {
	if query.Search != nil && len(query.Search.Terms) == 0 {
		return nil, nil
	}
	if query.Instances != nil && len(query.Instances) == 0 {
		return []Row{}, nil
	}
	rows, err := s.pageRows(ctx, query)
	if err != nil {
		if query.Search != nil {
			return nil, fmt.Errorf("search logs: %w", err)
		}
		return nil, fmt.Errorf("list logs: %w", err)
	}
	if query.Newer != nil {
		slices.Reverse(rows)
	}
	return rows, nil
}

func (s *Store) pageRows(ctx context.Context, query PageQuery) ([]Row, error) {
	after, through := pageKeyBounds(query)
	projectID, clientID, limit := query.Scope.ProjectID, query.Scope.ClientID, int32(query.Limit)
	switch {
	case query.Search == nil && query.Instances == nil:
		params := sqlc.ListLogsOlderParams{
			ProjectID: projectID, ClientID: clientID, PageSize: limit,
			AfterAt: after.at, AfterSequence: after.sequence, AfterID: after.id,
			ThroughAt: through.at, ThroughSequence: through.sequence, ThroughID: through.id,
		}
		if query.Newer != nil {
			return readRows(s.queries.ListLogsNewer(ctx, sqlc.ListLogsNewerParams(params)))
		}
		return readRows(s.queries.ListLogsOlder(ctx, params))
	case query.Search == nil:
		params := sqlc.ListLogsOlderByInstancesParams{
			ProjectID: projectID, ClientID: clientID, PageSize: limit,
			Instances: query.Instances,
			AfterAt:   after.at, AfterSequence: after.sequence, AfterID: after.id,
			ThroughAt: through.at, ThroughSequence: through.sequence, ThroughID: through.id,
		}
		if query.Newer != nil {
			return readRows(s.queries.ListLogsNewerByInstances(ctx, sqlc.ListLogsNewerByInstancesParams(params)))
		}
		return readRows(s.queries.ListLogsOlderByInstances(ctx, params))
	case query.Instances == nil:
		params := sqlc.SearchLogsOlderParams{
			ProjectID: projectID, ClientID: clientID, PageSize: limit,
			AfterAt: after.at, AfterSequence: after.sequence, AfterID: after.id,
			ThroughAt: through.at, ThroughSequence: through.sequence, ThroughID: through.id,
			PrimaryPattern: query.Search.PrimaryPattern, RemainingPatterns: query.Search.RemainingPatterns,
		}
		if query.Newer != nil {
			return readRows(s.search.SearchLogsNewer(ctx, sqlc.SearchLogsNewerParams(params)))
		}
		return readRows(s.search.SearchLogsOlder(ctx, params))
	default:
		params := sqlc.SearchLogsOlderByInstancesParams{
			ProjectID: projectID, ClientID: clientID, PageSize: limit,
			Instances: query.Instances,
			AfterAt:   after.at, AfterSequence: after.sequence, AfterID: after.id,
			ThroughAt: through.at, ThroughSequence: through.sequence, ThroughID: through.id,
			PrimaryPattern: query.Search.PrimaryPattern, RemainingPatterns: query.Search.RemainingPatterns,
		}
		if query.Newer != nil {
			return readRows(s.search.SearchLogsNewerByInstances(ctx, sqlc.SearchLogsNewerByInstancesParams(params)))
		}
		return readRows(s.search.SearchLogsOlderByInstances(ctx, params))
	}
}

type logReadKey struct {
	at       pgtype.Timestamptz
	sequence int64
	id       int64
}

// Intersect the time range and cursor into one bound per end of the index scan.
// A separate timestamp bound can cause PostgreSQL to start scanning before the
// full cursor key. Compare timestamps at pgx's microsecond encoding precision.
func pageKeyBounds(query PageQuery) (after, through logReadKey) {
	// SQL maps NULL timestamps to the corresponding infinities. Since sequences
	// are nonnegative, the minimum sequence sorts before every row at that time.
	after = logReadKey{sequence: math.MinInt64, id: math.MinInt64}
	through = logReadKey{sequence: math.MaxInt64, id: math.MaxInt64}
	if query.Range != nil {
		after.at = pgtype.Timestamptz{Time: query.Range.From.Truncate(time.Microsecond), Valid: true}
		through = logReadKey{
			at:       pgtype.Timestamptz{Time: query.Range.To.Truncate(time.Microsecond), Valid: true},
			sequence: math.MinInt64,
			id:       math.MinInt64,
		}
	}
	if cursor := query.Older; cursor != nil {
		at := cursor.OccurredAt.Truncate(time.Microsecond)
		if !through.at.Valid || at.Before(through.at.Time) {
			// Page validates positive cursor IDs. Decrementing the ID makes
			// the inclusive upper key exclude the cursor itself.
			through = logReadKey{at: pgtype.Timestamptz{Time: at, Valid: true}, sequence: cursor.Sequence, id: cursor.ID - 1}
		}
	}
	if cursor := query.Newer; cursor != nil {
		at := cursor.OccurredAt.Truncate(time.Microsecond)
		if !after.at.Valid || !at.Before(after.at.Time) {
			after = logReadKey{at: pgtype.Timestamptz{Time: at, Valid: true}, sequence: cursor.Sequence, id: cursor.ID}
		}
	}
	return after, through
}

// sqlc generates a separate type for each statement's shared column list.
type logReadRow = struct {
	ID             int64
	ClientID       uuid.UUID
	ProjectID      uuid.UUID
	InstanceName   string
	OccurredAt     time.Time
	SequenceNumber int64
	Message        string
}

func readRows[T ~logReadRow](found []T, err error) ([]Row, error) {
	if err != nil {
		return nil, err
	}
	rows := make([]Row, len(found))
	for i, row := range found {
		rows[i] = readRow(logReadRow(row))
	}
	return rows, nil
}

func readRow(row logReadRow) Row {
	return Row{
		ID: row.ID, Scope: Scope{ProjectID: row.ProjectID, ClientID: row.ClientID},
		Instance: row.InstanceName, OccurredAt: row.OccurredAt, Sequence: row.SequenceNumber, Message: row.Message,
	}
}

// Search selectivity varies per pattern. Cache parameter descriptions, but plan
// with the actual values so PostgreSQL can choose between B-tree and trigram indexes.
type searchQueryDB struct {
	sqlc.DBTX
}

func (db searchQueryDB) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return db.DBTX.Query(ctx, query, append([]any{pgx.QueryExecModeCacheDescribe}, args...)...)
}
