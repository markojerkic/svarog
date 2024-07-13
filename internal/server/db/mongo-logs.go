package db

import (
	"context"
	"fmt"
	"log"
	"log/slog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoLogRepository struct {
	mongoClient   *mongo.Client
	logCollection *mongo.Collection
}

var _ LogRepository = &MongoLogRepository{}

var clientsPipeline = mongo.Pipeline{
	bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$client.client_id"}}}},
	bson.D{{Key: "$project", Value: bson.D{{Key: "client_id", Value: "$_id"}}}},
}

// GetClients implements LogRepository.
func (self *MongoLogRepository) GetClients() ([]AvailableClient, error) {
	results, err := self.logCollection.Distinct(context.Background(), "client.client_id", bson.D{})
	if err != nil {
		return nil, err
	}

	clients := make([]AvailableClient, len(results))

	for i, result := range results {
		mappedClient := AvailableClient{
			Client: StoredClient{
				ClientId: result.(string),
			},
		}

		clients[i] = mappedClient
	}

	return clients, nil
}

// Can be useful if we want to drop clientId for some reason
var logsByClient = bson.D{}

func createFilter(clientId string, pageSize int64, lastCursor *LastCursor) (bson.D, *options.FindOptions) {
	sortDirection := -1

	projection := options.Find().SetProjection(logsByClient).SetLimit(pageSize).SetSort(bson.D{{Key: "timestamp", Value: sortDirection}, {Key: "sequence_number", Value: sortDirection}})

	clientIdFilter := bson.D{{Key: "client.client_id", Value: clientId}}
	var filter bson.D

	if lastCursor != nil && lastCursor.Timestamp.UnixMilli() > 0 {
		slog.Debug("Adding timestamp cursor", slog.Any("cursor", *lastCursor))

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

	return filter, projection
}

func (self *MongoLogRepository) getAndMapLogs(filter bson.D, projection *options.FindOptions) ([]StoredLog, error) {
	cursor, err := self.logCollection.Find(context.Background(), filter, projection)
	if err != nil {
		log.Printf("Error getting logs: %v\n", err)
		return nil, err
	}
	defer cursor.Close(context.Background())

	var logs []StoredLog
	if err = cursor.All(context.Background(), &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

// GetLogs implements LogRepository.
func (self *MongoLogRepository) GetLogs(clientId string, pageSize int64, lastCursor *LastCursor) ([]StoredLog, error) {
	slog.Debug(fmt.Sprintf("Getting logs for client %s", clientId))

	filter, projection := createFilter(clientId, pageSize, lastCursor)

	slog.Debug(fmt.Sprintf("Filter: %v", filter))
	return self.getAndMapLogs(filter, projection)
}

func (self *MongoLogRepository) SearchLogs(query string, clientId string, pageSize int64, lastCursor *LastCursor) ([]StoredLog, error) {
	slog.Debug(fmt.Sprintf("Getting logs for client %s", clientId))

	filter, projection := createFilter(clientId, pageSize, lastCursor)

	filter = append(filter, bson.E{Key: "$text", Value: bson.D{{Key: "$search", Value: query}}})
	// filter = bson.D{{"$text", bson.D{{"$search", query}}}}

	slog.Debug("Search, tu sam")
	slog.Debug("Search", slog.Any("filter", filter), slog.String("query", query))
	return self.getAndMapLogs(filter, projection)
}

func (self *MongoLogRepository) SaveLogs(logs []interface{}) error {
	_, err := self.logCollection.InsertMany(context.Background(), logs)
	if err != nil {
		slog.Error("Error saving logs", slog.Any("error", err))
		return err
	}

	return nil
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

func NewMongoClient(connectionUrl string) *MongoLogRepository {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(connectionUrl))
	if err != nil {
		log.Fatalf("Error connecting to mongo: %v", err)
	}
	collection := client.Database("logs").Collection("log_lines")

	repo := &MongoLogRepository{
		mongoClient:   client,
		logCollection: collection,
	}

	repo.createIndexes()

	return repo
}
