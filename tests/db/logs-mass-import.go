package db

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math"
	"testing"
	"time"

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
			slog.Info(fmt.Sprintf("Backlog still has %d items. Waiting 8s", suite.logServer.BacklogCount()))
			time.Sleep(8 * time.Second)
		} else {
			slog.Info("Backlog is empty, we can count items", slog.Int64("count", int64(suite.logServer.BacklogCount())))
			break
		}
	}

	clients, err := suite.logsRepository.GetClients(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(clients))

	count := suite.countNumberOfLogsInDb()
	slog.Info(fmt.Sprintf("Number of logs in db: %d", count))
	assert.Equal(t, numberOfImportedLogs, count)

	elapsed := time.Since(start)
	slog.Info(fmt.Sprintf("Imported %d logs in %s", numberOfImportedLogs, elapsed))
	suite.logServerContext.Done()

	// SECOND PART OF THE TEST
	// Check all logs if they're in correct order

	index := int(numberOfImportedLogs)
	pageSize := 200_000

	var lastCursorPtr *db.LastCursor
	for {
		logPage, err := suite.logsRepository.GetLogs(context.Background(), "marko", nil, int64(pageSize), lastCursorPtr)
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
