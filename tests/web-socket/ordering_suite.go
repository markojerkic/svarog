package websocket

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/markojerkic/svarog/internal/server/db"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type WebSocketOrderingSuite struct {
	testutils.BaseSuite

	logService *db.MongoLogService
	logServer  db.AggregatingLogServer

	logsCollection *mongo.Collection
	wsLogRenderer  *websocket.WsLogLineRenderer

	logServerContext       context.Context
	logServerContextCancel context.CancelFunc
}

func (suite *WebSocketOrderingSuite) SetupSuite() {
	suite.BaseSuite.SetupSuite()

	suite.wsLogRenderer = suite.WsLogRenderer
	suite.logService = db.NewLogService(suite.Database, suite.wsLogRenderer)
	suite.logsCollection = suite.Collection("log_lines")
}

func (suite *WebSocketOrderingSuite) SetupTest() {
	slog.Info("Setting up test. Creating log server and context")
	suite.logServerContext, suite.logServerContextCancel = context.WithCancel(context.Background())
	suite.logServer = db.NewLogServer(suite.logService)

	num := suite.countNumberOfLogsInDb()
	if ok := assert.Equal(suite.T(), int64(0), num, "Database should be empty before each test"); !ok {
		suite.T().FailNow()
	}
}

func (suite *WebSocketOrderingSuite) TearDownTest() {
	slog.Info("Tearing down test")

	// Cancel context and wait for log server to finish
	if suite.logServerContextCancel != nil {
		suite.logServerContextCancel()
	}

	result, err := suite.logsCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(suite.T(), err)

	slog.Info("Deleted logs", "count", result.DeletedCount)
	num := suite.countNumberOfLogsInDb()
	if ok := assert.Equal(suite.T(), int64(0), num, "Database teardown not successful"); !ok {
		suite.T().FailNow()
	}
}

func (suite *WebSocketOrderingSuite) TearDownSuite() {
	slog.Info("Tearing down suite")
	suite.BaseSuite.TearDownSuite()
}

func (suite *WebSocketOrderingSuite) countNumberOfLogsInDb() int64 {
	count, err := suite.logsCollection.CountDocuments(context.Background(), bson.D{})
	if err != nil {
		panic(fmt.Sprintf("Could not count documents: %v", err))
	}
	return count
}
