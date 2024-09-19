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

	container, err := mongodb.Run(suite.testContainerContext, "mongo:6")
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

	suite.mongoRepository = db.NewMongoClient(suite.connectionString)
	suite.logServer = db.NewLogServer(suite.logServerContext, suite.mongoRepository)

	mongoClient, err := mongo.Connect(context.Background(), options.Client().ApplyURI(suite.connectionString))
	suite.mongoClient = mongoClient
	if err != nil {
		log.Fatalf("Could not connect to mongo: %s", err)
	}
}

// Before each
func (suite *RepositorySuite) SetupTest() {
	slog.Info("Setting up test. Recreating context")
	suite.logServerContext = context.Background()
	suite.logServer = db.NewLogServer(suite.logServerContext, suite.mongoRepository)
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
