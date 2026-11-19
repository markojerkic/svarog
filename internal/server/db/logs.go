package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/markojerkic/svarog/internal/lib/logsearch"
	persistence "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/server/logstore"
)

type LastCursor struct {
	Timestamp      time.Time
	SequenceNumber int64
	ID             int64
	IsBackward     bool
}

type LogRange struct {
	From time.Time
	To   time.Time
}

type LogPage struct {
	Logs           []logstore.Row
	Instances      *[]string
	ForwardCursor  *LastCursor
	BackwardCursor *LastCursor
	IsLastPage     bool
}

type LogPageRequest struct {
	ProjectId string
	ClientId  string
	Instances *[]string
	PageSize  int64
	LogLineId *string
	Cursor    *LastCursor
	Search    string
	Range     *LogRange
}

type LogService interface {
	GetLogs(ctx context.Context, req LogPageRequest) (LogPage, error)
	GetInstances(ctx context.Context, projectId string, clientId string) ([]string, error)
	SearchLogs(ctx context.Context, query string, projectId string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]logstore.Row, error)
}

type PostgresLogService struct {
	pool *pgxpool.Pool
}

var _ LogService = &PostgresLogService{}

// GetInstances implements LogRepository.
func (self *PostgresLogService) GetInstances(ctx context.Context, projectId string, clientId string) ([]string, error) {
	projectID, clientID, ok := parseLogScope(projectId, clientId)
	if !ok {
		return []string{}, nil
	}
	instances, err := persistence.ListInstances(ctx, self.pool, projectID, clientID)
	if err != nil {
		return []string{}, err
	}
	return instances, nil
}

// GetLogs implements LogRepository.
func (self *PostgresLogService) GetLogs(ctx context.Context, req LogPageRequest) (LogPage, error) {
	projectID, clientID, ok := parseLogScope(req.ProjectId, req.ClientId)
	if !ok {
		return emptyLogPage(req), nil
	}
	if req.PageSize <= 0 || req.PageSize > math.MaxInt32 {
		return LogPage{}, fmt.Errorf("invalid log page size %d", req.PageSize)
	}
	if req.Search != "" {
		logs, err := self.searchLogs(ctx, req.Search, req.ProjectId, req.ClientId, req.Instances, req.PageSize, req.Cursor, req.Range)
		if err != nil {
			return LogPage{}, err
		}
		return pageFromLogs(logs, req), nil
	}
	if req.LogLineId != nil {
		anchor, found, err := self.logAnchor(ctx, projectID, clientID, *req.LogLineId)
		if err != nil {
			return LogPage{}, err
		}
		if found {
			req.Instances = &[]string{anchor.Instance}
			if req.Cursor == nil {
				return self.logContext(ctx, req, anchor)
			}
		} else {
			slog.Warn("Log context anchor not found", "logLineId", *req.LogLineId)
		}
	}

	query := persistence.LogReadQuery{
		ProjectID:       projectID,
		ClientID:        clientID,
		FilterInstances: req.Instances != nil,
		PageSize:        int32(req.PageSize),
	}
	if req.Instances != nil {
		query.Instances = *req.Instances
	}
	if req.Cursor != nil && req.Cursor.Timestamp.UnixMicro() > 0 {
		query.HasCursor = true
		query.Backward = req.Cursor.IsBackward
		query.CursorAt = req.Cursor.Timestamp
		query.CursorSequence = int64(req.Cursor.SequenceNumber)
		if req.Cursor.ID > 0 {
			id := req.Cursor.ID
			query.CursorID = &id
		}
	}
	applyLogRange(&query, req.Range)

	rows, err := persistence.ListLogs(ctx, self.pool, query)
	if err != nil {
		slog.Error("Error getting logs", "error", err)
		return LogPage{}, err
	}
	return pageFromLogs(rows, req), nil
}

func (self *PostgresLogService) logContext(ctx context.Context, req LogPageRequest, anchor logstore.Row) (LogPage, error) {
	query := persistence.LogReadQuery{
		ProjectID:       anchor.Scope.ProjectID,
		ClientID:        anchor.Scope.ClientID,
		FilterInstances: true,
		Instances:       *req.Instances,
		PageSize:        int32(req.PageSize),
		HasCursor:       true,
		Backward:        true,
		CursorAt:        anchor.OccurredAt,
		CursorSequence:  anchor.Sequence,
		CursorID:        &anchor.ID,
	}
	applyLogRange(&query, req.Range)
	older, err := persistence.ListLogs(ctx, self.pool, query)
	if err != nil {
		return LogPage{}, err
	}
	query.Backward = false
	newer, err := persistence.ListLogs(ctx, self.pool, query)
	if err != nil {
		return LogPage{}, err
	}
	page := LogPage{
		Logs:       append(append(newer, anchor), older...),
		Instances:  req.Instances,
		IsLastPage: len(newer) == 0,
	}
	if int64(len(older)) == req.PageSize {
		page.BackwardCursor = logCursor(page.Logs[len(page.Logs)-1], true)
	}
	if int64(len(newer)) == req.PageSize {
		page.ForwardCursor = logCursor(page.Logs[0], false)
	}
	return page, nil
}

func (self *PostgresLogService) SearchLogs(ctx context.Context, query string, projectId string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]logstore.Row, error) {
	return self.searchLogs(ctx, query, projectId, clientId, instances, pageSize, lastCursor, nil)
}

func (self *PostgresLogService) searchLogs(ctx context.Context, query string, projectId string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor, logRange *LogRange) ([]logstore.Row, error) {
	projectID, clientID, ok := parseLogScope(projectId, clientId)
	if !ok {
		return []logstore.Row{}, nil
	}
	if pageSize <= 0 || pageSize > math.MaxInt32 {
		return nil, fmt.Errorf("invalid log page size %d", pageSize)
	}
	parsed, err := logsearch.Parse(query)
	if err != nil {
		return nil, err
	}
	if len(parsed.Terms) == 0 {
		return []logstore.Row{}, nil
	}
	search := persistence.LogSearchQuery{
		LogReadQuery: persistence.LogReadQuery{
			ProjectID:       projectID,
			ClientID:        clientID,
			FilterInstances: instances != nil,
			PageSize:        int32(pageSize),
		},
		PrimaryPattern:    parsed.PrimaryPattern,
		RemainingPatterns: parsed.RemainingPatterns,
	}
	applyLogRange(&search.LogReadQuery, logRange)
	if instances != nil {
		search.Instances = *instances
	}
	if lastCursor != nil && lastCursor.Timestamp.UnixMicro() > 0 {
		search.HasCursor = true
		search.Backward = lastCursor.IsBackward
		search.CursorAt = lastCursor.Timestamp
		search.CursorSequence = int64(lastCursor.SequenceNumber)
		if lastCursor.ID > 0 {
			id := lastCursor.ID
			search.CursorID = &id
		}
	}
	rows, err := persistence.SearchLogs(ctx, self.pool, search)
	if err != nil {
		slog.Error("Error searching logs", "error", err)
		return nil, err
	}
	return rows, nil
}

// CommitLogs returns one result per input after a successful write.
func (self *PostgresLogService) CommitLogs(ctx context.Context, logs []logstore.Event) ([]persistence.IngestLogResult, error) {
	if len(logs) == 0 {
		return nil, nil
	}
	input := make([]persistence.IngestLogInsert, len(logs))
	for i, event := range logs {
		input[i] = persistence.IngestLogInsert{ProjectID: event.Scope.ProjectID, EventKey: event.Key, Log: persistence.LogInsert{
			ClientID:       event.Scope.ClientID,
			InstanceName:   event.Instance,
			OccurredAt:     event.OccurredAt,
			SequenceNumber: event.Sequence,
			Message:        event.Message,
		}}
	}
	return persistence.InsertIngestLogs(ctx, self.pool, input)
}

func NewLogService(pool *pgxpool.Pool) *PostgresLogService {
	return &PostgresLogService{pool: pool}
}

func parseLogScope(projectID, clientID string) (uuid.UUID, uuid.UUID, bool) {
	project, projectErr := uuid.Parse(projectID)
	client, clientErr := uuid.Parse(clientID)
	if projectErr != nil || clientErr != nil || project == uuid.Nil || client == uuid.Nil {
		return uuid.UUID{}, uuid.UUID{}, false
	}
	return project, client, true
}

func emptyLogPage(req LogPageRequest) LogPage {
	return LogPage{Instances: req.Instances, IsLastPage: req.Cursor == nil}
}

func applyLogRange(query *persistence.LogReadQuery, logRange *LogRange) {
	if logRange == nil {
		return
	}
	query.HasRange = true
	query.RangeFrom = logRange.From
	query.RangeTo = logRange.To
}

func (self *PostgresLogService) logAnchor(ctx context.Context, projectID, clientID uuid.UUID, logLineID string) (logstore.Row, bool, error) {
	id, err := strconv.ParseInt(logLineID, 10, 64)
	if err != nil || id <= 0 {
		return logstore.Row{}, false, nil
	}
	anchor, err := persistence.GetLogAnchor(ctx, self.pool, projectID, clientID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return logstore.Row{}, false, nil
	}
	if err != nil {
		return logstore.Row{}, false, err
	}
	return anchor, true, nil
}

func pageFromLogs(logs []logstore.Row, req LogPageRequest) LogPage {
	if len(logs) == 0 {
		return emptyLogPage(req)
	}
	var backwardCursor *LastCursor
	if int64(len(logs)) == req.PageSize && (req.Cursor == nil || req.Cursor.IsBackward) {
		backwardCursor = logCursor(logs[len(logs)-1], true)
	}
	var forwardCursor *LastCursor
	if int64(len(logs)) == req.PageSize && req.Cursor != nil && !req.Cursor.IsBackward {
		forwardCursor = logCursor(logs[0], false)
	}
	return LogPage{
		Logs:           logs,
		Instances:      req.Instances,
		ForwardCursor:  forwardCursor,
		BackwardCursor: backwardCursor,
		IsLastPage:     req.Cursor == nil,
	}
}

func logCursor(log logstore.Row, backward bool) *LastCursor {
	return &LastCursor{
		Timestamp:      log.OccurredAt,
		SequenceNumber: log.Sequence,
		ID:             log.ID,
		IsBackward:     backward,
	}
}
