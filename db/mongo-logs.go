package db

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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
	projection, err := self.logCollection.Aggregate(context.Background(), clientsPipeline)

	if err != nil {
		return nil, err
	}

	var results []StoredClient

	if err = projection.All(context.Background(), &results); err != nil {
		return nil, err
	}

	clients := make([]Client, len(results))

	fmt.Printf("Clients: %d\n", len(results))
	for _, result := range results {
		slog.Debug(fmt.Sprintf("Client: %v\n", result))
		clients = append(clients, Client{Client: result, IsOnline: false})
	}

	return clients, nil
}

var logsByClient = bson.D{{"log_line", 1}}

// GetLogs implements LogRepository.
func (self *MongoLogRepository) GetLogs(clientId string, lastCursor *time.Time) ([]StoredLog, error) {
	slog.Debug(fmt.Sprintf("Getting logs for client %s", clientId))

	projection := options.Find().SetProjection(logsByClient).SetLimit(50).SetSort(bson.D{{"timestamp", -1}})

	filter := bson.D{}
	if lastCursor != nil {
		filter = append(filter, bson.E{"timestamp", bson.D{{"$lt", *lastCursor}}})
	}

	cursor, err := self.logCollection.Find(context.Background(), bson.D{{"client.client_id", clientId}}, projection)
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
