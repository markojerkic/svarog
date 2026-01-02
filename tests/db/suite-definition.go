package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
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
	wsLogRenderer  *websocket.WsLogLineRenderer

	logServerContext context.Context
}

// Before all
func (suite *LogsCollectionRepositorySuite) SetupSuite() {
	suite.BaseSuite.SetupSuite()

	suite.logServerContext = context.Background()

	suite.wsLogRenderer = suite.WsLogRenderer
	suite.logService = db.NewLogService(suite.Database, suite.wsLogRenderer)
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

func (suite *LogsCollectionRepositorySuite) countNumberOfLogsInDb() int64 {
	count, err := suite.logsCollection.CountDocuments(context.Background(), bson.D{})
	if err != nil {
		panic(fmt.Sprintf("Could not count documents: %v", err))
	}
	return count
}

func generateLogLines(logIngestChannel chan<- db.LogLineWithHost, numberOfImportedLogs int64) {
	for i := 0; i < int(numberOfImportedLogs); i++ {
		logIngestChannel <- db.LogLineWithHost{
			LogLine: &rpc.LogLine{
				Message:   fmt.Sprintf("Log line %d", i),
				Timestamp: time.Now(),
				Sequence:  i,
			},
			ProjectId: "test-project",
			ClientId:  "marko",
			Hostname:  "::1",
		}

		if i%500_000 == 0 {
			slog.Info("Generated log lines", "count", i)
		}
	}
}
