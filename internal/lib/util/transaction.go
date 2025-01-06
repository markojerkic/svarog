package util

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

func StartTransaction(ctx context.Context, fn func(mongo.SessionContext) (interface{}, error), client *mongo.Client) (interface{}, error) {
	wc := writeconcern.Majority()
	tnxOptions := options.Transaction().SetWriteConcern(wc)
	session, err := client.StartSession()
	if err != nil {
		return struct{}{}, err
	}
	defer session.EndSession(ctx)

	return session.WithTransaction(ctx, fn, tnxOptions)
}
