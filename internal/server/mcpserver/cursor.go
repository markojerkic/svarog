package mcpserver

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/logsearch"
	"github.com/markojerkic/svarog/internal/server/live"
	"github.com/markojerkic/svarog/internal/server/logstore"
)

const (
	defaultLogLimit = 50
	maxLogLimit     = 200
	cursorVersion   = 1
)

// Unbounded ends of a one-sided time range. PostgreSQL stores both.
var (
	earliestLogTime = time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	latestLogTime   = time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)
)

var errInvalidCursor = errors.New("cursor is invalid; use older_cursor or newer_cursor from a previous result")

// logQuery holds the filters that a cursor repeats on every page.
type logQuery struct {
	Search    string   `json:"q,omitempty"`
	Instances []string `json:"in,omitempty"` // empty selects all instances
	FromUs    *int64   `json:"f,omitempty"`  // Unix microseconds, inclusive
	ToUs      *int64   `json:"t,omitempty"`  // Unix microseconds, exclusive
	Limit     int      `json:"l"`
}

// logCursor is opaque to clients. It carries no authority: every page
// re-checks the user's access to its project and client.
type logCursor struct {
	Version   int       `json:"v"`
	ProjectID uuid.UUID `json:"p"`
	ClientID  uuid.UUID `json:"c"`
	Newer     bool      `json:"n,omitempty"`
	AtUs      int64     `json:"at"`
	Sequence  int64     `json:"s"`
	ID        int64     `json:"id"`
	logQuery
}

func newLogQuery(search string, instances []string, from, to string, limit int) (logQuery, error) {
	query := logQuery{Search: strings.TrimSpace(search), Limit: limit}
	if query.Limit == 0 {
		query.Limit = defaultLogLimit
	}
	if len(instances) > 0 {
		query.Instances = instances
	}
	var err error
	if query.FromUs, err = parseTimestamp("from", from); err != nil {
		return logQuery{}, err
	}
	if query.ToUs, err = parseTimestamp("to", to); err != nil {
		return logQuery{}, err
	}
	if _, err := query.pageQuery(logstore.Scope{}); err != nil {
		return logQuery{}, err
	}
	return query, nil
}

func parseTimestamp(name, value string) (*int64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, errors.New(name + " must be an RFC 3339 timestamp, for example 2026-01-02T15:04:05Z")
	}
	us := parsed.UnixMicro()
	return &us, nil
}

// pageQuery validates the filters and converts them to a store query.
func (q logQuery) pageQuery(scope logstore.Scope) (logstore.PageQuery, error) {
	if q.Limit < 1 || q.Limit > maxLogLimit {
		return logstore.PageQuery{}, errors.New("limit must be between 1 and 200")
	}
	query := logstore.PageQuery{Scope: scope, Limit: q.Limit}
	if q.Search != "" {
		search, err := logsearch.Parse(q.Search)
		if errors.Is(err, logsearch.ErrTooShort) {
			return logstore.PageQuery{}, errors.New("query needs at least one term with three consecutive letters or digits")
		}
		if err != nil {
			return logstore.PageQuery{}, err
		}
		query.Search = &search
	}
	if len(q.Instances) > 0 {
		// The live view's limits bound every instance selection.
		selector, err := live.NewSelector(q.Instances)
		if err != nil {
			return logstore.PageQuery{}, errors.New("instances must list at most 32 names from list_instances")
		}
		query.Instances = selector.Instances
	}
	if q.FromUs != nil || q.ToUs != nil {
		bounds := logstore.TimeRange{From: earliestLogTime, To: latestLogTime}
		if q.FromUs != nil {
			bounds.From = time.UnixMicro(*q.FromUs)
		}
		if q.ToUs != nil {
			bounds.To = time.UnixMicro(*q.ToUs)
		}
		if !bounds.From.Before(bounds.To) {
			return logstore.PageQuery{}, errors.New("from must be earlier than to")
		}
		query.Range = &bounds
	}
	return query, nil
}

func encodeCursor(scope logstore.Scope, query logQuery, newer bool, position logstore.Cursor) string {
	data, err := json.Marshal(logCursor{
		Version: cursorVersion, ProjectID: scope.ProjectID, ClientID: scope.ClientID, Newer: newer,
		AtUs: position.OccurredAt.UnixMicro(), Sequence: position.Sequence, ID: position.ID, logQuery: query,
	})
	if err != nil {
		// Every field has a fixed JSON encoding.
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(encoded string) (logCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return logCursor{}, errInvalidCursor
	}
	var cursor logCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.Version != cursorVersion || cursor.ID <= 0 ||
		cursor.ProjectID == uuid.Nil || cursor.ClientID == uuid.Nil {
		return logCursor{}, errInvalidCursor
	}
	return cursor, nil
}

func (c logCursor) position() *logstore.Cursor {
	return &logstore.Cursor{OccurredAt: time.UnixMicro(c.AtUs), Sequence: c.Sequence, ID: c.ID}
}

func rowPosition(row logstore.Row) logstore.Cursor {
	return logstore.Cursor{OccurredAt: row.OccurredAt, Sequence: row.Sequence, ID: row.ID}
}
