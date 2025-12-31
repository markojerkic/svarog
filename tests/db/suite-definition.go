package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type LogsCollectionRepositorySuite struct {
	suite.Suite
	mongoContainer *testutils.MongoDBTestContainer

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

	util.SetupLogger()

	mongoContainer, err := testutils.NewMongoDBTestContainer(suite.testContainerContext, "svarog")
	if err != nil {
		panic(fmt.Errorf("could not start MongoDB container: %w", err))
	}
	suite.mongoContainer = mongoContainer

	suite.logService = db.NewLogService(mongoContainer.Database)
	suite.logServer = db.NewLogServer(suite.logService)
	suite.logsCollection = mongoContainer.Database.Collection("log_lines")
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
	result, err := suite.logsCollection.DeleteMany(context.Background(), bson.M{})

	assert.NoError(suite.T(), err)

	slog.Info("Deleted logs", "count", result.DeletedCount)
	num := suite.countNumberOfLogsInDb()
	if ok := assert.Equal(suite.T(), int64(0), num, "Database teardown not successful"); !ok {
		suite.T().FailNow()
	}
	suite.logServerContext.Done()
}

// After all
func (suite *LogsCollectionRepositorySuite) TearDownSuite() {
	slog.Info("Tearing down suite")
	if err := suite.mongoContainer.Terminate(suite.testContainerContext); err != nil {
		panic(fmt.Sprintf("failed to terminate container: %s", err))
	}
}
