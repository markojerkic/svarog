package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/logsearch"
	persistence "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/server/live"
	"github.com/markojerkic/svarog/internal/server/types"
)

type LastCursor struct {
	Timestamp      time.Time
	SequenceNumber int
	ID             int64
	IsBackward     bool
}

type LogRange struct {
	From     time.Time
	To       time.Time
	FromDate string
	ToDate   string
	Offset   int
}

type LogPage struct {
	Logs           []types.StoredLog
	Instances      *[]string
	ForwardCursor  *LastCursor
	BackwardCursor *LastCursor
	IsLastPage     bool
	Range          *LogRange
}

func (l *LogPage) ToPath(projectId, clientId string, instances *[]string, cursor *LastCursor, direction, search string) string {
	u := url.URL{
		Path: fmt.Sprintf("/logs/%s/%s", projectId, clientId),
	}
	query := u.Query()
	if cursor != nil {
		query.Set("cursorTimeUs", strconv.FormatInt(cursor.Timestamp.UnixMicro(), 10))
		query.Set("cursorSequenceNumber", fmt.Sprintf("%d", cursor.SequenceNumber))
		query.Set("direction", direction)
		if cursor.ID > 0 {
			query.Set("cursorId", strconv.FormatInt(cursor.ID, 10))
		}
	}
	if instances != nil {
		for _, instance := range *instances {
			query.Add("instance", instance)
		}
	}
	if search != "" {
		query.Set("search", search)
	}
	if l.Range != nil {
		query.Set("from", l.Range.FromDate)
		query.Set("to", l.Range.ToDate)
		query.Set("offset", strconv.Itoa(l.Range.Offset))
	}
	u.RawQuery = query.Encode()
	return u.String()
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
	SearchLogs(ctx context.Context, query string, projectId string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]types.StoredLog, error)
}

type PostgresLogService struct {
	pool     *pgxpool.Pool
	liveLogs *live.Logs
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
			req.Instances = &[]string{anchor.InstanceName}
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
	return pageFromLogs(mapLogRows(rows), req), nil
}

func (self *PostgresLogService) logContext(ctx context.Context, req LogPageRequest, anchor persistence.LogRow) (LogPage, error) {
	query := persistence.LogReadQuery{
		ProjectID:       anchor.ProjectID,
		ClientID:        anchor.ClientID,
		FilterInstances: true,
		Instances:       *req.Instances,
		PageSize:        int32(req.PageSize),
		HasCursor:       true,
		Backward:        true,
		CursorAt:        anchor.OccurredAt,
		CursorSequence:  anchor.SequenceNumber,
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
		Logs:       mapLogRows(append(append(newer, anchor), older...)),
		Instances:  req.Instances,
		IsLastPage: len(newer) == 0,
		Range:      req.Range,
	}
	if int64(len(older)) == req.PageSize {
		page.BackwardCursor = logCursor(page.Logs[len(page.Logs)-1], true)
	}
	if int64(len(newer)) == req.PageSize {
		page.ForwardCursor = logCursor(page.Logs[0], false)
	}
	return page, nil
}

func (self *PostgresLogService) SearchLogs(ctx context.Context, query string, projectId string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]types.StoredLog, error) {
	return self.searchLogs(ctx, query, projectId, clientId, instances, pageSize, lastCursor, nil)
}

func (self *PostgresLogService) searchLogs(ctx context.Context, query string, projectId string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor, logRange *LogRange) ([]types.StoredLog, error) {
	projectID, clientID, ok := parseLogScope(projectId, clientId)
	if !ok {
		return []types.StoredLog{}, nil
	}
	if pageSize <= 0 || pageSize > math.MaxInt32 {
		return nil, fmt.Errorf("invalid log page size %d", pageSize)
	}
	parsed, err := logsearch.Parse(query)
	if err != nil {
		return nil, err
	}
	if len(parsed.Terms) == 0 {
		return []types.StoredLog{}, nil
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
	return mapLogRows(rows), nil
}

var ErrInvalidLogClientID = errors.New("invalid log client id")

// CommitLogs returns nil after inserting new events or recognizing an existing
// event key. Live delivery is best effort and must never turn a commit into a retry.
func (self *PostgresLogService) CommitLogs(ctx context.Context, logs []IngestLog) error {
	if len(logs) == 0 {
		return nil
	}
	input := make([]persistence.IngestLogInsert, len(logs))
	for i, event := range logs {
		log := event.Log
		clientID, err := uuid.Parse(log.Client.ClientId)
		if err != nil || clientID == uuid.Nil || clientID.String() != log.Client.ClientId {
			return fmt.Errorf("%w %q", ErrInvalidLogClientID, log.Client.ClientId)
		}
		input[i] = persistence.IngestLogInsert{EventKey: event.EventKey, Log: persistence.LogInsert{
			ClientID:       clientID,
			InstanceName:   log.Client.InstanceId,
			OccurredAt:     log.Timestamp,
			SequenceNumber: int64(log.SequenceNumber),
			Message:        log.LogLine,
		}}
	}
	inserted, err := persistence.InsertIngestLogs(ctx, self.pool, input)
	if err != nil {
		slog.Error("Error saving logs", "error", err)
		return err
	}
	newRows := make([]types.StoredLog, 0, len(inserted))
	for _, row := range inserted {
		log := logs[row.Ordinal-1].Log
		log.ID = row.ID
		newRows = append(newRows, log)
	}
	if self.liveLogs != nil {
		self.liveLogs.AfterCommit(ctx, newRows)
	}
	return nil
}

func NewLogService(pool *pgxpool.Pool, liveLogs *live.Logs) *PostgresLogService {
	return &PostgresLogService{
		pool:     pool,
		liveLogs: liveLogs,
	}
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
	return LogPage{Instances: req.Instances, IsLastPage: req.Cursor == nil, Range: req.Range}
}

func applyLogRange(query *persistence.LogReadQuery, logRange *LogRange) {
	if logRange == nil {
		return
	}
	query.HasRange = true
	query.RangeFrom = logRange.From
	query.RangeTo = logRange.To
}

// ParseLogRange turns inclusive local calendar dates into a half-open UTC
// interval. offsetMin is JavaScript's getTimezoneOffset: minutes to add to
// local time to get UTC.
func ParseLogRange(fromDate, toDate string, offsetMin int) (LogRange, error) {
	if offsetMin < -14*60 || offsetMin > 14*60 {
		return LogRange{}, fmt.Errorf("invalid timezone offset")
	}
	fromDay, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return LogRange{}, fmt.Errorf("invalid from date")
	}
	toDay, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		return LogRange{}, fmt.Errorf("invalid to date")
	}
	if toDay.Before(fromDay) {
		fromDay, toDay = toDay, fromDay
		fromDate, toDate = toDate, fromDate
	}
	offset := time.Duration(offsetMin) * time.Minute
	return LogRange{
		From:     fromDay.Add(offset),
		To:       toDay.AddDate(0, 0, 1).Add(offset),
		FromDate: fromDate,
		ToDate:   toDate,
		Offset:   offsetMin,
	}, nil
}

func (self *PostgresLogService) logAnchor(ctx context.Context, projectID, clientID uuid.UUID, logLineID string) (persistence.LogRow, bool, error) {
	id, err := strconv.ParseInt(logLineID, 10, 64)
	if err != nil || id <= 0 {
		return persistence.LogRow{}, false, nil
	}
	anchor, err := persistence.GetLogAnchor(ctx, self.pool, projectID, clientID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return persistence.LogRow{}, false, nil
	}
	if err != nil {
		return persistence.LogRow{}, false, err
	}
	return anchor, true, nil
}

func mapLogRows(rows []persistence.LogRow) []types.StoredLog {
	logs := make([]types.StoredLog, len(rows))
	for i, row := range rows {
		logs[i] = types.StoredLog{
			ID:             row.ID,
			LogLine:        row.Message,
			Timestamp:      row.OccurredAt,
			SequenceNumber: int(row.SequenceNumber),
			Client: types.StoredClient{
				ProjectId:  row.ProjectID.String(),
				ClientId:   row.ClientID.String(),
				InstanceId: row.InstanceName,
			},
		}
	}
	return logs
}

func pageFromLogs(logs []types.StoredLog, req LogPageRequest) LogPage {
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
		Range:          req.Range,
	}
}

func logCursor(log types.StoredLog, backward bool) *LastCursor {
	return &LastCursor{
		Timestamp:      log.Timestamp,
		SequenceNumber: log.SequenceNumber,
		ID:             log.ID,
		IsBackward:     backward,
	}
}
