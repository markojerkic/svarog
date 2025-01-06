package db

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/server/types"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoLogRepository struct {
	logCollection *mongo.Collection
}

var _ LogRepository = &MongoLogRepository{}

var instancesPipeline = mongo.Pipeline{
	bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$client.client_id"}}}},
	bson.D{{Key: "$project", Value: bson.D{{Key: "ip_address", Value: "$_id"}}}},
}

// GetInstances implements LogRepository.
func (self *MongoLogRepository) GetInstances(ctx context.Context, clientId string) ([]string, error) {
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

// GetClients implements LogRepository.
func (self *MongoLogRepository) GetClients(ctx context.Context) ([]AvailableClient, error) {
	results, err := self.logCollection.Distinct(ctx, "client.client_id", bson.D{})
	if err != nil {
		return nil, err
	}

	clients := make([]AvailableClient, len(results))

	for i, result := range results {
		mappedClient := AvailableClient{
			Client: types.StoredClient{
				ClientId: result.(string),
			},
		}

		clients[i] = mappedClient
	}

	return clients, nil
}

// GetLogs implements LogRepository.
func (self *MongoLogRepository) GetLogs(ctx context.Context, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]types.StoredLog, error) {
	log.Debug("Getting logs for client", "client", clientId)

	filter, projection := createFilter(clientId, pageSize, instances, lastCursor)

	log.Debug("Filter logs", "filter", filter)
	return self.getAndMapLogs(ctx, filter, projection)
}

func (self *MongoLogRepository) WatchInserts(ctx context.Context) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "operationType", Value: "insert"},
		}}},
	}

	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	changeStream, err := self.logCollection.Watch(ctx, pipeline, opts)

	if err != nil {
		log.Fatalf("Error watching inserts: %v", err)
	}

	defer changeStream.Close(ctx)

	for changeStream.Next(ctx) {
		var event bson.M
		if err := changeStream.Decode(&event); err != nil {
			log.Error("Error decoding log", "error", err)
		}
		fullDocument := event["fullDocument"].(bson.M)

		var storedLog types.StoredLog
		bsonBytes, err := bson.Marshal(fullDocument) // Convert bson.M to bytes
		if err != nil {
			log.Error("Error marshalling log", "error", err)
			continue
		}

		err = bson.Unmarshal(bsonBytes, &storedLog) // Unmarshal into the Person struct
		if err != nil {
			log.Error("Error unmarshalling log", "error", err)
			continue
		}

		websocket.LogsHub.NotifyInsert(storedLog)
	}
}

func (self *MongoLogRepository) SearchLogs(ctx context.Context, query string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]types.StoredLog, error) {
	log.Debug("Getting logs for client", "clientId", clientId)

	filter, projection := createFilter(clientId, pageSize, instances, lastCursor)

	filter = append(filter, bson.E{Key: "$text", Value: bson.D{{Key: "$search", Value: query}}})

	return self.getAndMapLogs(ctx, filter, projection)
}

func (self *MongoLogRepository) SaveLogs(ctx context.Context, logs []types.StoredLog) error {
	saveableLogs := make([]interface{}, len(logs))
	for i, log := range logs {
		saveableLogs[i] = log
	}
	insertedLines, err := self.logCollection.InsertMany(ctx, saveableLogs)
	if err != nil {
		log.Error("Error saving logs", "error", err)
		return err
	}

	for i := range logs {
		logs[i].ID = insertedLines.InsertedIDs[i].(primitive.ObjectID)
	}

	return nil
}

func NewLogRepository(db *mongo.Database) *MongoLogRepository {
	collection := db.Collection("log_lines")

	repo := &MongoLogRepository{
		logCollection: collection,
	}

	repo.createIndexes()
	go repo.WatchInserts(context.Background())

	return repo
}

// Can be useful if we want to drop clientId for some reason
var logsByClient = bson.D{}

func createFilter(clientId string, pageSize int64, instances *[]string, lastCursor *LastCursor) (bson.D, *options.FindOptions) {
	sortDirection := -1

	projection := options.Find().SetProjection(logsByClient).SetLimit(pageSize).SetSort(bson.D{{Key: "timestamp", Value: sortDirection}, {Key: "sequence_number", Value: sortDirection}})

	clientIdFilter := bson.D{{Key: "client.client_id", Value: clientId}}
	var filter bson.D

	if lastCursor != nil && lastCursor.Timestamp.UnixMilli() > 0 {
		log.Debug("Adding timestamp cursor", "cursor", *lastCursor)

		timestamp := primitive.NewDateTimeFromTime(lastCursor.Timestamp)

		var direction string

		if lastCursor.IsBackward {
			direction = "$lt"
		} else {
			direction = "$gt"
		}

		filter = bson.D{
			{Key: "client.client_id", Value: clientId},
			{Key: "$or", Value: bson.A{
				bson.D{
					{Key: "timestamp", Value: bson.D{{Key: direction, Value: timestamp}}},
				},
				bson.D{
					{Key: "timestamp", Value: timestamp},
					{Key: "sequence_number", Value: bson.D{{Key: direction, Value: lastCursor.SequenceNumber}}},
				},
			}},
		}

	} else {
		filter = clientIdFilter
	}

	if instances != nil {
		filter = append(filter, bson.E{
			Key: "client.ip_address",
			Value: bson.D{
				{Key: "$in", Value: instances},
			},
		})
	}

	return filter, projection
}

func (self *MongoLogRepository) getAndMapLogs(ctx context.Context, filter bson.D, projection *options.FindOptions) ([]types.StoredLog, error) {
	cursor, err := self.logCollection.Find(ctx, filter, projection)
	if err != nil {
		log.Printf("Error getting logs: %v\n", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []types.StoredLog
	if err = cursor.All(ctx, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (self *MongoLogRepository) createIndexes() {
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
		log.Fatalf("Error creating indexes: %v", err)
	}
}
