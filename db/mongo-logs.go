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
func (self *MongoLogRepository) GetClients() ([]Client, error) {
	results, err := self.logCollection.Distinct(context.Background(), "client.client_id", bson.D{})

	if err != nil {
		return nil, err
	}
	clients := make([]Client, 0, len(results))

	for _, result := range results {
		mappedClient := Client{
			Client: StoredClient{
				ClientId: result.(string),
			},
		}

		clients = append(clients, mappedClient)
	}

	return clients, nil
}

var logsByClient = bson.D{{"_id", 1}, {"log_line", 1}, {"timestamp", 1}}

// GetLogs implements LogRepository.
func (self *MongoLogRepository) GetLogs(clientId string, lastCursor *LastCursor) ([]StoredLog, error) {
	slog.Debug(fmt.Sprintf("Getting logs for client %s", clientId))

	sortDirection := -1

	projection := options.Find().SetProjection(logsByClient).SetLimit(300).SetSort(bson.D{{"timestamp", sortDirection}, {"sequence_number", sortDirection}})

	clientIdFilter := bson.D{{"client.client_id", clientId}}
	var filter bson.D

	if lastCursor != nil && lastCursor.Timestamp.UnixMilli() > 0 {
		slog.Debug("Adding timestamp cursor", slog.Any("cursor", *lastCursor))

		objectId, err := primitive.ObjectIDFromHex(lastCursor.ID)
		if err != nil {
			return nil, err
		}
		timestamp := primitive.NewDateTimeFromTime(lastCursor.Timestamp)

		var direction string

		if lastCursor.IsBackward {
			direction = "$lt"
		} else {
			direction = "$gt"
		}

		filter = bson.D{
			{"client.client_id", clientId},
			{"timestamp", bson.D{{direction, timestamp}}},
			{"_id", bson.D{{direction, objectId}}},
		}

	} else {
		filter = clientIdFilter
	}

	slog.Debug(fmt.Sprintf("Filter: %v", filter))

	cursor, err := self.logCollection.Find(context.Background(), filter, projection)
	// cursor, err := self.logCollection.Find(context.Background(), filter, projection)
	if err != nil {
		log.Printf("Error getting logs: %v\n", err)
		return nil, err
	}
	defer cursor.Close(context.Background())

	var logs []StoredLog
	if err = cursor.All(context.Background(), &logs); err != nil {
		return nil, err
	}

	slog.Debug(fmt.Sprintf("Found %d log lines", len(logs)), slog.String("clientId", clientId))

	return logs, nil
}

func (self *MongoLogRepository) SaveLogs(logs []interface{}) error {
	saved, err := self.logCollection.InsertMany(context.Background(), logs)
	if err != nil {
		slog.Error("Error saving logs: %v", err)
		return err
	}

	slog.Debug(fmt.Sprintf("Saved %d log lines\n", len(saved.InsertedIDs)))

	return nil
}

func NewMongoClient(connectionUrl string) *MongoLogRepository {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(connectionUrl))
	if err != nil {
		panic(err)
	}
	collection := client.Database("logs").Collection("log_lines")

	return &MongoLogRepository{
		mongoClient:   client,
		logCollection: collection,
	}
}
