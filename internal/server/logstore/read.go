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
	names := make([]string, len(instances))
	for i, instance := range instances {
		names[i] = instance.InstanceName
	}
	return names, nil
}

// InstanceHues returns only the requested colors, spaced around the color wheel
// according to the complete client's alphabetical palette, independent of filters.
func (s *Store) InstanceHues(ctx context.Context, scope Scope, names []string) (map[string]float64, error) {
	palette := make(map[string]float64, len(names))
	if len(names) == 0 {
		return palette, nil
	}
	rows, err := s.queries.ListInstances(ctx, sqlc.ListInstancesParams{
		ProjectID: scope.ProjectID, ClientID: scope.ClientID, VisibleNames: names,
	})
	if err != nil {
		return nil, fmt.Errorf("read instance colors: %w", err)
	}
	for _, row := range rows {
		palette[row.InstanceName] = row.Hue
	}
	return palette, nil
}

const instancePageSize = 5

func (s *Store) InstanceOptions(ctx context.Context, scope Scope, term, unnamedLabel string, after *InstanceCursor) (InstanceOptions, error) {
	params := sqlc.ListInstanceOptionsParams{
		ClientID: scope.ClientID, ProjectID: scope.ProjectID, PageSize: instancePageSize + 1,
		Term: term, UnnamedLabel: unnamedLabel,
	}
	if after != nil {
		day, err := time.Parse(time.DateOnly, after.Day)
		if err != nil {
			return InstanceOptions{}, err
		}
		params.AfterDay, params.AfterName = pgtype.Date{Time: day, Valid: true}, after.Name
	}
	rows, err := s.queries.ListInstanceOptions(ctx, params)
	if err != nil {
		return InstanceOptions{}, fmt.Errorf("list instance options: %w", err)
	}
	page := InstanceOptions{Names: make([]string, min(len(rows), instancePageSize))}
	for i := range page.Names {
		page.Names[i] = rows[i].InstanceName
	}
	if len(rows) > instancePageSize {
		last := rows[instancePageSize-1]
		page.Next = &InstanceCursor{Day: last.LastDay.Time.Format(time.DateOnly), Name: last.InstanceName}
	}
	return page, nil
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
	case query.Search != nil:
		return s.searchPage(ctx, query, after, through)
	case query.Instances == nil:
		params := sqlc.ListLogsOlderParams{
			ProjectID: projectID, ClientID: clientID, PageSize: limit,
			AfterAt: after.at, AfterSequence: after.sequence, AfterID: after.id,
			ThroughAt: through.at, ThroughSequence: through.sequence, ThroughID: through.id,
		}
		if query.Newer != nil {
			return readRows(s.queries.ListLogsNewer(ctx, sqlc.ListLogsNewerParams(params)))
		}
		return readRows(s.queries.ListLogsOlder(ctx, params))
	default:
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
	}
}

func (s *Store) searchPage(ctx context.Context, query PageQuery, after, through logReadKey) ([]Row, error) {
	params := sqlc.SearchLogsParams{
		ProjectID: query.Scope.ProjectID, ClientID: query.Scope.ClientID, PageSize: int32(query.Limit),
		AfterAt: after.at, AfterSequence: after.sequence, AfterID: after.id,
		ThroughAt: through.at, ThroughSequence: through.sequence, ThroughID: through.id,
		PrimaryPattern: query.Search.PrimaryPattern, RemainingPatterns: query.Search.RemainingPatterns,
		Instances: query.Instances, Probe: query.Range == nil, Newer: query.Newer != nil,
	}
	search := sqlc.New(searchQueryDB{DBTX: s.pool})
	rows, err := readRows(search.SearchLogs(ctx, params))
	if err != nil || !params.Probe || len(rows) == query.Limit {
		return rows, err
	}

	// A partial probe cannot rule out later matches. Read the whole page from
	// one snapshot, including any rows that matched in the probe.
	var hits []sqlc.SearchLogHitsRow
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// The client and trigram bitmaps can exceed the default 4 MB.
		_, err := tx.Exec(ctx, "SET LOCAL work_mem = '32MB'")
		if err != nil {
			return err
		}
		search := sqlc.New(searchQueryDB{DBTX: tx})
		hits, err = search.SearchLogHits(ctx, sqlc.SearchLogHitsParams{
			ProjectID: params.ProjectID, ClientID: params.ClientID, PageSize: params.PageSize,
			AfterAt: after.at, AfterSequence: after.sequence, AfterID: after.id,
			ThroughAt: through.at, ThroughSequence: through.sequence, ThroughID: through.id,
			PrimaryPattern: params.PrimaryPattern, RemainingPatterns: params.RemainingPatterns,
			Instances: params.Instances, Newer: params.Newer,
		})
		return err
	})
	return readRows(hits, err)
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
