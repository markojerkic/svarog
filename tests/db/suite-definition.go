package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LogsCollectionRepositorySuite struct {
	suite.Suite
	container        *mongodb.MongoDBContainer
	connectionString string

	logService *db.MongoLogService
	logServer  db.AggregatingLogServer

	logsCollection *mongo.Collection

	testContainerContext context.Context
	logServerContext     context.Context
}

// Before all
func (suite *LogsCollectionRepositorySuite) SetupSuite() {
	suite.logServerContext = context.Background()
	suite.testContainerContext = context.Background()

	container, err := mongodb.Run(suite.testContainerContext, "mongo:7", mongodb.WithReplicaSet("rs0"))
	if err != nil {
		panic(fmt.Errorf("Could not start container: %w", err))
	}

	suite.container = container
	suite.connectionString, err = container.ConnectionString(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Could not get connection string: %s", err))
	}

	util.SetupLogger()
	

	clientOptions := options.Client().ApplyURI(suite.connectionString)
	mongoClient, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		suite.T().Fatal(err)
	}
	suite.initiateReplicaSet(mongoClient)

	database := mongoClient.Database("svarog")

	suite.logService = db.NewLogService(database)
	suite.logServer = db.NewLogServer(suite.logService)
	suite.logsCollection = database.Collection("log_lines")

	if err != nil {
		panic(fmt.Sprintf("Could not connect to mongo: %s", err))
	}
}

func (self *LogsCollectionRepositorySuite) initiateReplicaSet(client *mongo.Client) error {
	host, err := self.container.Host(context.Background())

	if err != nil {
		return err
	}
	options := bson.D{
		{Key: "replSetInitiate", Value: bson.D{
			{Key: "_id", Value: "rs0"},
			{Key: "members", Value: bson.A{bson.D{
				{Key: "_id", Value: 0},
				{Key: "host", Value: host},
			},
			}},
		}},
	}

	result := client.Database("admin").RunCommand(context.Background(), options)
	if result.Err() != nil {
		return result.Err()
	}

	return nil
}

// Before each
func (suite *LogsCollectionRepositorySuite) SetupTest() {
	slog.Info("Setting up test. Recreating context")
	suite.logServer = db.NewLogServer(suite.logService)
	num := suite.countNumberOfLogsInDb()
	if ok := assert.Equal(suite.T(), int64(0), num, "Database should be empty before each test"); !ok {
		suite.T().FailNow()
	}
}

// After each
func (suite *LogsCollectionRepositorySuite) TearDownTest() {
	slog.Info("Tearing down test")
	// err := suite.mongoClient.Database("logs").Collection("log_lines").Drop(context.Background())
	result, err := suite.logsCollection.DeleteMany(context.Background(), bson.M{})

	assert.NoError(suite.T(), err)

	slog.Info("Deleted logs: ", "count", result.DeletedCount)
	num := suite.countNumberOfLogsInDb()
	if ok := assert.Equal(suite.T(), int64(0), num, "Database teardown not successful"); !ok {
		suite.T().FailNow()
	}
	suite.logServerContext.Done()
}

// After all
func (suite *LogsCollectionRepositorySuite) TearDownSuite() {
	slog.Info("Tearing down suite")
	if err := suite.container.Terminate(suite.testContainerContext); err != nil {
		panic(fmt.Sprintf("failed to terminate container: %s", err))
	}
}
