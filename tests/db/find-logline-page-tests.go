package db

import (
	"context"
	"time"

	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log/slog"
)

func (suite *LogsCollectionRepositorySuite) TestFindLoglinePage() {
	t := suite.T()
	expectedCount := int64(10_000)

	logIngestChannel := make(chan db.LogLineWithHost, 1024)

	logServerContext := context.Background()
	defer logServerContext.Done()

	go suite.logServer.Run(logServerContext, logIngestChannel)

	generateLogLines(logIngestChannel, expectedCount)

	// Wait for all logs to be inserted into the database
	timeout := time.After(30 * time.Second)
	for {
		count := suite.countNumberOfLogsInDb()
		if count >= expectedCount {
			slog.Info("All logs inserted", "count", count)
			break
		}

		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for logs to be inserted. Expected %d, got %d", expectedCount, count)
		default:
			slog.Info("Waiting for logs to be inserted", "current", count, "expected", expectedCount)
			time.Sleep(1 * time.Second)
		}
	}

	randomLogLine, err := suite.findRandomLogLine()
	assert.NoError(t, err)

	// Find logline page
	logLineId := randomLogLine.ID.Hex()
	logPage, err := suite.logService.GetLogs(logServerContext, db.LogPageRequest{
		ProjectId: "test-project",
		ClientId:  randomLogLine.Client.ClientId,
		Instances: nil,
		PageSize:  300,
		LogLineId: &logLineId,
		Cursor:    nil,
	})
	assert.NoError(t, err)

	// Assert logs page contains randomLogLine and rest of the items are correctly sorted
	assert.Equal(t, 300, len(logPage.Logs))
	assert.True(t, findLogLine(logPage.Logs, logLineId) >= 0)
}

func findLogLine(logs []types.StoredLog, logLineId string) int {
	for i, log := range logs {
		if log.ID.Hex() == logLineId {
			return i
		}
	}
	return -1
}

// Find logline with default sorting at index 5_000
func (suite *LogsCollectionRepositorySuite) findRandomLogLine() (*types.StoredLog, error) {
	var logLine types.StoredLog
	sort := bson.D{{Key: "timestamp", Value: -1}, {Key: "sequence_number", Value: -1}}
	err := suite.logsCollection.FindOne(context.Background(), bson.D{}, options.FindOne().SetSort(sort).SetSkip(5_000)).Decode(&logLine)
	return &logLine, err
}
