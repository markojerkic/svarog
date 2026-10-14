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
	persistence "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/server/types"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type LastCursor struct {
	Timestamp      time.Time
	SequenceNumber int
	ID             int64
	IsBackward     bool
}

type LogPage struct {
	Logs           []types.StoredLog
	ForwardCursor  *LastCursor
	BackwardCursor *LastCursor
	IsLastPage     bool
}

func (l *LogPage) ToPath(projectId, clientId string, instanceId *string, cursor *LastCursor, direction string) string {
	u := url.URL{
		Path: fmt.Sprintf("/logs/%s/%s", projectId, clientId),
	}
	query := u.Query()
	if cursor != nil {
		query.Set("cursorTime", fmt.Sprintf("%d", cursor.Timestamp.UnixMilli()))
		query.Set("cursorSequenceNumber", fmt.Sprintf("%d", cursor.SequenceNumber))
		query.Set("direction", direction)
		if cursor.ID > 0 {
			query.Set("cursorId", strconv.FormatInt(cursor.ID, 10))
		}
	}
	if instanceId != nil {
		query.Set("instance", *instanceId)
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
}

type LogService interface {
	SaveLogs(ctx context.Context, logs []types.StoredLog) error
	GetLogs(ctx context.Context, req LogPageRequest) (LogPage, error)
	GetInstances(ctx context.Context, projectId string, clientId string) ([]string, error)
	SearchLogs(ctx context.Context, query string, projectId string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]types.StoredLog, error)
	DeleteLogBeforeTimestamp(ctx context.Context, timestamp time.Time) error
}

type MongoLogService struct {
	logCollection *mongo.Collection
	pool          *pgxpool.Pool
	wsLogRenderer *websocket.WsLogLineRenderer
}

var _ LogService = &MongoLogService{}

// GetInstances implements LogRepository.
func (self *MongoLogService) GetInstances(ctx context.Context, projectId string, clientId string) ([]string, error) {
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

// DeleteLogAfterTimestamp implements LogService.
func (self *MongoLogService) DeleteLogBeforeTimestamp(ctx context.Context, timestamp time.Time) error {
	deleteResult, err := self.logCollection.DeleteMany(ctx, bson.D{{Key: "timestamp", Value: bson.D{{
		Key:   "$lte",
		Value: primitive.NewDateTimeFromTime(timestamp),
	}}}})
	slog.Debug("Deleting logs before timestamp", "timestamp", timestamp, "deleted", deleteResult.DeletedCount)
	if err != nil {
		slog.Error("Error deleting logs", "error", err)
		return err
	}

	return nil
}

// GetLogs implements LogRepository.
func (self *MongoLogService) GetLogs(ctx context.Context, req LogPageRequest) (LogPage, error) {
	projectID, clientID, ok := parseLogScope(req.ProjectId, req.ClientId)
	if !ok {
		return emptyLogPage(req), nil
	}
	if req.PageSize <= 0 || req.PageSize > math.MaxInt32 {
		return LogPage{}, fmt.Errorf("invalid log page size %d", req.PageSize)
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
	if req.Cursor != nil && req.Cursor.Timestamp.UnixMilli() > 0 {
		query.HasCursor = true
		query.Backward = req.Cursor.IsBackward
		query.CursorAt = req.Cursor.Timestamp
		query.CursorSequence = int64(req.Cursor.SequenceNumber)
		if req.Cursor.ID > 0 {
			id := req.Cursor.ID
			query.CursorID = &id
		}
	} else if req.LogLineId != nil {
		anchor, found, err := self.logAnchor(ctx, projectID, clientID, *req.LogLineId)
		if err != nil {
			return LogPage{}, err
		}
		if found {
			query.HasCursor = true
			query.Backward = true
			query.Inclusive = true
			query.CursorAt = anchor.OccurredAt
			query.CursorSequence = anchor.SequenceNumber
			query.CursorID = &anchor.ID
		} else {
			slog.Error("Failed to create filter for logLineId", "logLineId", *req.LogLineId)
		}
	}

	rows, err := persistence.ListLogs(ctx, self.pool, query)
	if err != nil {
		slog.Error("Error getting logs", "error", err)
		return LogPage{}, err
	}
	return pageFromLogs(mapLogRows(rows), req), nil
}

func (self *MongoLogService) SearchLogs(ctx context.Context, query string, projectId string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]types.StoredLog, error) {
	projectID, clientID, ok := parseLogScope(projectId, clientId)
	if !ok {
		return []types.StoredLog{}, nil
	}
	if pageSize <= 0 || pageSize > math.MaxInt32 {
		return nil, fmt.Errorf("invalid log page size %d", pageSize)
	}
	search := persistence.LogSearchQuery{
		LogReadQuery: persistence.LogReadQuery{
			ProjectID:       projectID,
			ClientID:        clientID,
			FilterInstances: instances != nil,
			PageSize:        int32(pageSize),
		},
		Search: query,
	}
	if instances != nil {
		search.Instances = *instances
	}
	if lastCursor != nil && lastCursor.Timestamp.UnixMilli() > 0 {
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

func (self *MongoLogService) SaveLogs(ctx context.Context, logs []types.StoredLog) error {
	if len(logs) == 0 {
		return nil
	}
	input := make([]persistence.LogInsert, len(logs))
	for i, log := range logs {
		clientID, err := uuid.Parse(log.Client.ClientId)
		if err != nil || clientID == uuid.Nil || clientID.String() != log.Client.ClientId {
			return fmt.Errorf("invalid log client id %q", log.Client.ClientId)
		}
		input[i] = persistence.LogInsert{
			ClientID:       clientID,
			InstanceName:   log.Client.InstanceId,
			OccurredAt:     log.Timestamp,
			SequenceNumber: int64(log.SequenceNumber),
			Message:        log.LogLine,
		}
	}
	ids, err := persistence.InsertLogs(ctx, self.pool, input)
	if err != nil {
		slog.Error("Error saving logs", "error", err)
		return err
	}
	for i := range logs {
		logs[i].ID = ids[i]
		self.wsLogRenderer.Render(ctx, logs[i])
	}
	return nil
}

func NewLogService(db *mongo.Database, pool *pgxpool.Pool, wsLogRenderer *websocket.WsLogLineRenderer) *MongoLogService {
	repo := &MongoLogService{
		pool:          pool,
		wsLogRenderer: wsLogRenderer,
	}
	if db == nil {
		return repo
	}
	repo.logCollection = db.Collection("log_lines")
	repo.createIndexes()
	return repo
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
	return LogPage{IsLastPage: req.Cursor == nil}
}

func (self *MongoLogService) logAnchor(ctx context.Context, projectID, clientID uuid.UUID, logLineID string) (persistence.LogAnchor, bool, error) {
	id, err := strconv.ParseInt(logLineID, 10, 64)
	if err != nil || id <= 0 {
		return persistence.LogAnchor{}, false, nil
	}
	anchor, err := persistence.GetLogAnchor(ctx, self.pool, projectID, clientID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return persistence.LogAnchor{}, false, nil
	}
	if err != nil {
		return persistence.LogAnchor{}, false, err
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
	if req.Cursor == nil || req.Cursor.IsBackward {
		last := logs[len(logs)-1]
		backwardCursor = &LastCursor{
			Timestamp:      last.Timestamp,
			SequenceNumber: last.SequenceNumber,
			ID:             last.ID,
			IsBackward:     true,
		}
	}
	var forwardCursor *LastCursor
	if req.Cursor != nil && !req.Cursor.IsBackward {
		first := logs[0]
		forwardCursor = &LastCursor{
			Timestamp:      first.Timestamp,
			SequenceNumber: first.SequenceNumber,
			ID:             first.ID,
			IsBackward:     false,
		}
	}
	return LogPage{
		Logs:           logs,
		ForwardCursor:  forwardCursor,
		BackwardCursor: backwardCursor,
		IsLastPage:     req.Cursor == nil,
	}
}

func (self *MongoLogService) createIndexes() {
	_, err := self.logCollection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "client.project_id", Value: 1},
				{Key: "client.client_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "client.project_id", Value: 1},
				{Key: "client.client_id", Value: 1},
				{Key: "timestamp", Value: -1},
				{Key: "sequence_number", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "log_line", Value: "text"},
				{Key: "timestamp", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "client.project_id", Value: 1},
				{Key: "client.client_id", Value: 1},
				{Key: "client.instance_id", Value: 1},
			},
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Error creating indexes: %v", err))
	}
}
