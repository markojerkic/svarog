package db

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (suite *LogsCollectionRepositorySuite) countNumberOfLogsInDb() int64 {
	collection := suite.logsCollection

	count, err := collection.CountDocuments(context.Background(), bson.D{})
	if err != nil {
		log.Fatalf("Could not count documents: %v", err)
		panic(err)
	}
	return count
}

func generateLogLines(logIngestChannel chan<- db.LogLineWithIp, numberOfImportedLogs int64) {
	for i := 0; i < int(numberOfImportedLogs); i++ {
		logIngestChannel <- db.LogLineWithIp{
			LogLine: &rpc.LogLine{
				Message:   fmt.Sprintf("Log line %d", i),
				Timestamp: timestamppb.New(time.Now()),
				Sequence:  int64(i) % math.MaxInt64,
				Client:    "marko",
			},
			Ip: "::1",
		}

		if i%500_000 == 0 {
			log.Printf("Generated %d log lines", i)
		}
	}
}

var numberOfImportedLogs = int64(1_000_000)

func (suite *LogsCollectionRepositorySuite) TestMassImport() {
	t := suite.T()
	start := time.Now()

	logIngestChannel := make(chan db.LogLineWithIp, 1024)

	logServerContext := context.Background()
	defer logServerContext.Done()

	go suite.logServer.Run(logServerContext, logIngestChannel)
	generateLogLines(logIngestChannel, numberOfImportedLogs)

	for {
		if !suite.logServer.IsBacklogEmpty() {
			log.Info("Backlog still has items. Waiting 8s", "numItem", suite.logServer.BacklogCount())
			time.Sleep(8 * time.Second)
		} else {
			log.Info("Backlog is empty, we can count items", "count", int64(suite.logServer.BacklogCount()))
			break
		}
	}

	clients, err := suite.logService.GetClients(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(clients))

	count := suite.countNumberOfLogsInDb()
	log.Info("Number of logs in db", "count", count)
	assert.Equal(t, numberOfImportedLogs, count)

	elapsed := time.Since(start)
	log.Info(fmt.Sprintf("Imported %d logs in %s", numberOfImportedLogs, elapsed))
	suite.logServerContext.Done()

	// SECOND PART OF THE TEST
	// Check all logs if they're in correct order

	index := int(numberOfImportedLogs)
	pageSize := 200_000

	var lastCursorPtr *db.LastCursor
	for {
		logPage, err := suite.logService.GetLogs(context.Background(), "marko", nil, int64(pageSize), nil, lastCursorPtr)
		assert.NoError(t, err)
		lastCursorPtr = validateLogListIsRightOrder(logPage, index, t)
		index -= pageSize
		if index <= 0 || lastCursorPtr == nil {
			break
		}
	}

	assert.LessOrEqual(t, index, 0, "Finished checking logs prematurely")

}

func validateLogListIsRightOrder(logPage []types.StoredLog, i int, t *testing.T) *db.LastCursor {
	for _, log := range logPage {
		ok := assert.Equal(t, fmt.Sprintf("Log line %d", i-1), log.LogLine)
		if !ok {
			t.FailNow()
		}
		i--
	}

	if len(logPage) == 0 {
		return nil
	}

	lastLogLine := logPage[len(logPage)-1]

	return &db.LastCursor{
		SequenceNumber: int(lastLogLine.SequenceNumber),
		Timestamp:      lastLogLine.Timestamp,
		IsBackward:     true,
	}
}
