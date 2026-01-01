package db

import (
	"context"
	"fmt"
	"time"

	"github.com/markojerkic/svarog/internal/server/types"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log/slog"
)

type LastCursor struct {
	Timestamp      time.Time
	SequenceNumber int
	IsBackward     bool
}

type LogPage struct {
	Logs           []types.StoredLog
	ForwardCursor  *LastCursor
	BackwardCursor *LastCursor
	IsLastPage     bool
}

type LogPageRequest struct {
	ClientId  string
	Instances *[]string
	PageSize  int64
	LogLineId *string
	Cursor    *LastCursor
}

type LogService interface {
	SaveLogs(ctx context.Context, logs []types.StoredLog) error
	GetLogs(ctx context.Context, req LogPageRequest) (LogPage, error)
	GetInstances(ctx context.Context, clientId string) ([]string, error)
	SearchLogs(ctx context.Context, query string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]types.StoredLog, error)
	DeleteLogBeforeTimestamp(ctx context.Context, timestamp time.Time) error
}

type MongoLogService struct {
	logCollection *mongo.Collection
	wsLogRenderer *websocket.WsLogLineRenderer
}

var _ LogService = &MongoLogService{}

// GetInstances implements LogRepository.
func (self *MongoLogService) GetInstances(ctx context.Context, clientId string) ([]string, error) {
	rawInstances, err := self.logCollection.Distinct(ctx, "client.ip_address", bson.D{{Key: "client.client_id", Value: clientId}})
	if err != nil {
		return []string{}, err
	}

	instances := make([]string, len(rawInstances))
	for i, instance := range rawInstances {
		instances[i] = instance.(string)
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
	slog.Debug("Getting logs for client", "client", req.ClientId)

	filter, projection := createFilter(self.logCollection, req)

	slog.Debug("Filter logs", "filter", filter)
	logs, err := self.getAndMapLogs(ctx, filter, projection)
	if err != nil {
		return LogPage{}, err
	}

	if len(logs) == 0 {
		return LogPage{
			Logs:           logs,
			ForwardCursor:  nil,
			BackwardCursor: nil,
			IsLastPage:     true,
		}, nil
	}

	// BackwardCursor: for scrolling up (older logs)
	backwardCursor := &LastCursor{
		Timestamp:      logs[len(logs)-1].Timestamp,
		SequenceNumber: logs[len(logs)-1].SequenceNumber,
		IsBackward:     true,
	}
	var forwardCursor *LastCursor
	if req.Cursor != nil {
		forwardCursor = &LastCursor{
			Timestamp:      logs[0].Timestamp,
			SequenceNumber: logs[0].SequenceNumber,
			IsBackward:     false,
		}
	}

	return LogPage{
		Logs:           logs,
		ForwardCursor:  forwardCursor,
		BackwardCursor: backwardCursor,
		IsLastPage:     forwardCursor == nil,
	}, nil
}

func (self *MongoLogService) SearchLogs(ctx context.Context, query string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]types.StoredLog, error) {
	slog.Debug("Getting logs for client", "clientId", clientId)

	filter, projection := createFilter(self.logCollection, LogPageRequest{
		ClientId:  clientId,
		Instances: instances,
		PageSize:  pageSize,
		LogLineId: nil,
		Cursor:    lastCursor,
	})

	filter = append(filter, bson.E{Key: "$text", Value: bson.D{{Key: "$search", Value: query}}})

	return self.getAndMapLogs(ctx, filter, projection)
}

func (self *MongoLogService) SaveLogs(ctx context.Context, logs []types.StoredLog) error {
	saveableLogs := make([]any, len(logs))
	for i, log := range logs {
		saveableLogs[i] = log
	}
	insertedLines, err := self.logCollection.InsertMany(
		ctx,
		saveableLogs,
		options.InsertMany().SetOrdered(false),
	)
	if err != nil {
		slog.Error("Error saving logs", "error", err)
		return err
	}

	for i := range logs {
		logs[i].ID = insertedLines.InsertedIDs[i].(primitive.ObjectID)
		self.wsLogRenderer.Render(ctx, logs[i])
	}

	return nil
}

func NewLogService(db *mongo.Database, wsLogRenderer *websocket.WsLogLineRenderer) *MongoLogService {
	collection := db.Collection("log_lines")

	repo := &MongoLogService{
		logCollection: collection,
		wsLogRenderer: wsLogRenderer,
	}

	repo.createIndexes()

	return repo
}

func createFilter(collection *mongo.Collection, req LogPageRequest) (bson.D, *options.FindOptions) {
	sortDirection := -1

	projection := options.Find().SetLimit(req.PageSize).SetSort(bson.D{{Key: "timestamp", Value: sortDirection}, {Key: "sequence_number", Value: sortDirection}})

	clientIdFilter := bson.D{{Key: "client.client_id", Value: req.ClientId}}
	var filter bson.D

	if req.Cursor != nil && req.Cursor.Timestamp.UnixMilli() > 0 {
		slog.Debug("Adding timestamp cursor", "cursor", *req.Cursor)

		timestamp := primitive.NewDateTimeFromTime(req.Cursor.Timestamp)

		var direction string

		if req.Cursor.IsBackward {
			direction = "$lt"
		} else {
			direction = "$gt"
		}

		filter = bson.D{
			{Key: "client.client_id", Value: req.ClientId},
			{Key: "$or", Value: bson.A{
				bson.D{
					{Key: "timestamp", Value: bson.D{{Key: direction, Value: timestamp}}},
				},
				bson.D{
					{Key: "timestamp", Value: timestamp},
					{Key: "sequence_number", Value: bson.D{{Key: direction, Value: req.Cursor.SequenceNumber}}},
				},
			}},
		}

	} else if req.LogLineId != nil {
		// Find page of data where logLineId is in the middle of the page
		slog.Debug("Adding log line id cursor", "logLineId", *req.LogLineId)
		newFilter, newProjection, err := createFilterForLogLine(collection, req.ClientId, *req.LogLineId, req.PageSize)
		if err != nil {
			slog.Error("Failed to create filter for logLineId", "error", err)
			filter = clientIdFilter
		} else {
			filter = newFilter
			projection = newProjection
		}

	} else {
		filter = clientIdFilter
	}

	if req.Instances != nil {
		filter = append(filter, bson.E{
			Key: "client.ip_address",
			Value: bson.D{
				{Key: "$in", Value: req.Instances},
			},
		})
	}

	return filter, projection
}

func (self *MongoLogService) getAndMapLogs(ctx context.Context, filter bson.D, projection *options.FindOptions) ([]types.StoredLog, error) {
	cursor, err := self.logCollection.Find(ctx, filter, projection)
	if err != nil {
		slog.Error("Error getting logs", "error", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []types.StoredLog
	if err = cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func createFilterForLogLine(collection *mongo.Collection, clientId string, logLineId string, pageSize int64) (bson.D, *options.FindOptions, error) {
	sortDirection := -1
	projection := options.Find().SetSort(bson.D{
		{Key: "timestamp", Value: sortDirection},
		{Key: "sequence_number", Value: sortDirection},
	})

	// Convert logLineId to ObjectID
	logId, err := primitive.ObjectIDFromHex(logLineId)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid logLineId format: %w", err)
	}

	// Find the target log to get its timestamp and sequence_number
	var targetLog struct {
		Timestamp      primitive.DateTime `bson:"timestamp"`
		SequenceNumber int64              `bson:"sequence_number"`
	}
	err = collection.FindOne(context.Background(), bson.D{
		{Key: "_id", Value: logId},
		{Key: "client.client_id", Value: clientId},
	}).Decode(&targetLog)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find target log: %w", err)
	}

	// Calculate half page size (for logs before and after the target)
	halfPageSize := pageSize - 1

	// Set limit to half page size + 1 (including the target log)
	projection.SetLimit(halfPageSize + 1)

	// Create a filter that will find logs around the target log
	filter := bson.D{
		{Key: "client.client_id", Value: clientId},
		{Key: "$or", Value: bson.A{
			bson.D{
				{Key: "timestamp", Value: bson.D{{Key: "$lt", Value: targetLog.Timestamp}}},
			},
			bson.D{
				{Key: "timestamp", Value: targetLog.Timestamp},
				{Key: "sequence_number", Value: bson.D{{Key: "$lte", Value: targetLog.SequenceNumber}}},
			},
		}},
	}

	return filter, projection, nil
}

func (self *MongoLogService) createIndexes() {
	_, err := self.logCollection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "client.client_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "client.client_id", Value: 1}, {Key: "timestamp", Value: -1}, {Key: "sequence_number", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "log_line", Value: "text"}, {Key: "timestamp", Value: -1}},
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Error creating indexes: %v", err))
	}
}
