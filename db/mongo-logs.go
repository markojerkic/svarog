package db

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoLogRepository struct {
	mongoClient   *mongo.Client
	logCollection *mongo.Collection
}

var _ LogRepository = &MongoLogRepository{}

var projection = options.Find().SetProjection(bson.D{{"log_line", 1}})

// GetLogs implements LogRepository.
func (self *MongoLogRepository) GetLogs() ([]StoredLog, error) {

	cursor, err := self.logCollection.Find(context.Background(), bson.D{}, projection)
	if err != nil {
		log.Printf("Error getting logs: %v\n", err)
		return nil, err
	}

	var logs []StoredLog
	if err = cursor.All(context.Background(), &logs); err != nil {
		return nil, err
	}

	return logs, nil
}

func (self *MongoLogRepository) SaveLogs(logs []interface{}) error {
	saved, err := self.logCollection.InsertMany(context.Background(), logs)
	if err != nil {
		return err
	}

	log.Printf("Saved %d log lines\n", len(saved.InsertedIDs))

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
