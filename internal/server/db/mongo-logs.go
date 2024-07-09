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
	bson.D{{"$group", bson.D{{"_id", "$client.client_id"}}}},
	bson.D{{"$project", bson.D{{"client_id", "$_id"}}}},
}

// GetClients implements LogRepository.
func (self *MongoLogRepository) GetClients() ([]AvailableClient, error) {
	results, err := self.logCollection.Distinct(context.Background(), "client.client_id", bson.D{})

	if err != nil {
		return nil, err
	}
	clients := make([]AvailableClient, 0, len(results))

	for _, result := range results {
		mappedClient := AvailableClient{
			Client: StoredClient{
				ClientId: result.(string),
			},
		}

		clients = append(clients, mappedClient)
	}

	return clients, nil
}

// Can be useful if we want to drop clientId for some reason
var logsByClient = bson.D{}

func createFilter(clientId string, pageSize int64, lastCursor *LastCursor) (bson.D, *options.FindOptions) {
	sortDirection := -1

	projection := options.Find().SetProjection(logsByClient).SetLimit(pageSize).SetSort(bson.D{{"timestamp", sortDirection}, {"sequence_number", sortDirection}})

	clientIdFilter := bson.D{{"client.client_id", clientId}}
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
			{"client.client_id", clientId},
			{"$or", bson.A{
				bson.D{
					{"timestamp", bson.D{{direction, timestamp}}},
				},
				bson.D{
					{"timestamp", timestamp},
					{"sequence_number", bson.D{{direction, lastCursor.SequenceNumber}}},
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

	filter = append(filter, bson.E{"$text", bson.D{{"$search", query}}})
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
			Keys: bson.D{{"client.client_id", 1}},
		},
		{
			Keys: bson.D{{"client.client_id", 1}, {"timestamp", -1}, {"sequence_number", -1}},
		},
		{
			Keys: bson.D{{"log_line", "text"}, {"timestamp", -1}},
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
