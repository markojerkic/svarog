package db

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RepositorySuite struct {
	suite.Suite
	container        *mongodb.MongoDBContainer
	connectionString string

	mongoRepository *db.MongoLogRepository
	logServer       db.AggregatingLogServer

	mongoClient *mongo.Client

	testContainerContext context.Context
	logServerContext     context.Context
}

// Before all
func (suite *RepositorySuite) SetupSuite() {
	suite.logServerContext = context.Background()
	suite.testContainerContext = context.Background()

	container, err := mongodb.Run(suite.testContainerContext, "mongo:7", mongodb.WithReplicaSet("rs0"))
	if err != nil {
		log.Fatalf("Could not start container: %s", err)
	}

	suite.container = container
	suite.connectionString, err = container.ConnectionString(context.Background())
	if err != nil {
		log.Fatalf("Could not get connection string: %s", err)
	}

	logOpts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(os.Stdout, logOpts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	clientOptions := options.Client().ApplyURI(suite.connectionString)
	mongoClient, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		suite.T().Fatal(err)
	}
	database := mongoClient.Database("svarog")

	suite.mongoRepository = db.NewLogRepository(database)
	suite.logServer = db.NewLogServer(suite.mongoRepository)

	mongoClient, err = mongo.Connect(context.Background(), options.Client().ApplyURI(suite.connectionString))
	if err != nil {
		suite.T().Fatal(err)
	}
	suite.mongoClient = mongoClient

	suite.initiateReplicaSet(mongoClient)

	if err != nil {
		log.Fatalf("Could not connect to mongo: %s", err)
	}
}

func (self *RepositorySuite) initiateReplicaSet(client *mongo.Client) error {
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
func (suite *RepositorySuite) SetupTest() {
	slog.Info("Setting up test. Recreating context")
	suite.logServer = db.NewLogServer(suite.mongoRepository)
	num := suite.countNumberOfLogsInDb()
	if ok := assert.Equal(suite.T(), int64(0), num, "Database should be empty before each test"); !ok {
		suite.T().FailNow()
	}
}

// After each
func (suite *RepositorySuite) TearDownTest() {
	slog.Info("Tearing down test")
	// err := suite.mongoClient.Database("logs").Collection("log_lines").Drop(context.Background())
	result, err := suite.mongoClient.Database("logs").Collection("log_lines").DeleteMany(context.Background(), bson.M{})

	assert.NoError(suite.T(), err)

	slog.Info("Deleted logs: ", slog.Any("count", result.DeletedCount))
	num := suite.countNumberOfLogsInDb()
	if ok := assert.Equal(suite.T(), int64(0), num, "Database teardown not successful"); !ok {
		suite.T().FailNow()
	}
	suite.logServerContext.Done()
}

// After all
func (suite *RepositorySuite) TearDownSuite() {
	log.Println("Tearing down suite")
	if err := suite.container.Terminate(suite.testContainerContext); err != nil {
		log.Fatalf("failed to terminate container: %s", err)
	}
}
