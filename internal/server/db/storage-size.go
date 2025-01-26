package db

import (
	"context"

	"github.com/charmbracelet/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (self *MongoLogRepository) GetStorageSizeForClients(ctx context.Context, clientIds []string) (float64, error) {
	matchStage := bson.D{{Key: "$match", Value: bson.D{{Key: "client.client_id", Value: bson.D{{Key: "$in", Value: clientIds}}}}}}
	addFieldsStage := bson.D{{Key: "$addFields", Value: bson.D{{Key: "documentSize", Value: bson.D{{Key: "$bsonSize", Value: "$$ROOT"}}}}}}
	groupStage := bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$client.client_id"}, {Key: "totalSizeBytes", Value: bson.D{{Key: "$sum", Value: "$documentSize"}}}}}}

	cursor, err := self.logCollection.Aggregate(ctx, mongo.Pipeline{matchStage, addFieldsStage, groupStage})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		TotalSizeBytes float64 `bson:"totalSizeBytes"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return 0, err
	}

	var totalSize float64
	for _, result := range results {
		totalSize += result.TotalSizeBytes
	}
	totalSize = totalSize / 1024 / 1024

	log.Debug("Total size for clients", "size", totalSize, "clientIds", clientIds)

	return totalSize, nil
}
