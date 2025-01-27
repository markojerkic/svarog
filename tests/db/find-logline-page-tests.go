package db

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (suite *LogsCollectionRepositorySuite) TestFindLoglinePage() {
	t := suite.T()

	logIngestChannel := make(chan db.LogLineWithIp, 1024)

	logServerContext := context.Background()
	defer logServerContext.Done()

	go suite.logServer.Run(logServerContext, logIngestChannel)

	generateLogLines(logIngestChannel, 10_000)

	for {
		if !suite.logServer.IsBacklogEmpty() {
			log.Info("Backlog still has %d items. Waiting 6s", suite.logServer.BacklogCount())
			time.Sleep(6 * time.Second)
		} else {
			log.Info("Backlog is empty, we can count items", "count", int64(suite.logServer.BacklogCount()))
			break
		}
	}
	randomLogLine, err := suite.findRandomLogLine()
	assert.NoError(t, err)

	// Find logline page
	logLineId := randomLogLine.ID.Hex()
	logs, err := suite.logService.GetLogs(logServerContext, randomLogLine.Client.ClientId, nil, 300, &logLineId, nil)
	assert.NoError(t, err)

	// Asserrt logs page contains randomLogLine and rest of the items are correctly sorted
	assert.Equal(t, 300, len(logs))
	assert.True(t, findLogLine(logs, logLineId) >= 0)
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
