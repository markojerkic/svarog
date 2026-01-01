package db

import (
	"context"
	"log/slog"

	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type LogsCollectionRepositorySuite struct {
	testutils.BaseSuite

	logService *db.MongoLogService
	logServer  db.AggregatingLogServer

	logsCollection *mongo.Collection

	logServerContext context.Context
}

// Before all
func (suite *LogsCollectionRepositorySuite) SetupSuite() {
	config := testutils.DefaultBaseSuiteConfig()
	config.EnableNats = false
	suite.WithConfig(config)

	suite.BaseSuite.SetupSuite()

	suite.logServerContext = context.Background()

	suite.logService = db.NewLogService(suite.Database)
	suite.logServer = db.NewLogServer(suite.logService)
	suite.logsCollection = suite.Collection("log_lines")
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
	suite.BaseSuite.TearDownSuite()
}
