package testutils

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDBTestContainer holds the MongoDB testcontainer and its connection details
type MongoDBTestContainer struct {
	Container        *mongodb.MongoDBContainer
	ConnectionString string
	Client           *mongo.Client
	Database         *mongo.Database
}

// NewMongoDBTestContainer creates and starts a MongoDB testcontainer with replica set
func NewMongoDBTestContainer(ctx context.Context, databaseName string) (*MongoDBTestContainer, error) {
	container, err := mongodb.Run(ctx, "mongo:7", mongodb.WithReplicaSet("rs0"))
	if err != nil {
		return nil, fmt.Errorf("could not start MongoDB container: %w", err)
	}

	connectionString, err := container.ConnectionString(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("could not get connection string: %w", err)
	}

	clientOptions := options.Client().ApplyURI(connectionString)
	mongoClient, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("could not connect to MongoDB: %w", err)
	}

	tc := &MongoDBTestContainer{
		Container:        container,
		ConnectionString: connectionString,
		Client:           mongoClient,
		Database:         mongoClient.Database(databaseName),
	}


	return tc, nil
}

// Terminate cleans up the MongoDB testcontainer
func (tc *MongoDBTestContainer) Terminate(ctx context.Context) error {
	if tc.Client != nil {
		_ = tc.Client.Disconnect(ctx)
	}
	if tc.Container != nil {
		return tc.Container.Terminate(ctx)
	}
	return nil
}
