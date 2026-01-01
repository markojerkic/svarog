package outoforder

import (
	"context"
	"fmt"
	"testing"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestOutOfOrderSuite(t *testing.T) {
	suite.Run(t, new(OutOfOrderSuite))
}

func generateOddAndEvenLines(logIngestChannel chan<- db.LogLineWithHost, numberOfImportedLogs int64) {
	generatedLogLines := make([]*rpc.LogLine, numberOfImportedLogs)

	for i := 0; i < int(numberOfImportedLogs); i++ {
		generatedLogLines[i] = &rpc.LogLine{
			Message:   fmt.Sprintf("Log line %d", i),
			Timestamp: time.Now(),
			Sequence:  i,
		}
		if i%2_000 == 0 {
			slog.Info("Generated log lines", "count", i)
		}
	}

	i := 0
	for i < int(numberOfImportedLogs) {
		if i%1000 == 0 {
			slog.Debug("Sending even line", "index", i)
		}
		logIngestChannel <- db.LogLineWithHost{LogLine: generatedLogLines[i], ClientId: "marko", Hostname: "::1"}
		i += 2
	}
	slog.Info("Done with even lines")

	if int(numberOfImportedLogs) != len(generatedLogLines) {
		panic("Expected matching log counts")
	}

	i = 1
	for i < int(numberOfImportedLogs) {
		if i%1000 == 0 {
			slog.Debug("Sending odd line", "index", i)
		}
		logIngestChannel <- db.LogLineWithHost{LogLine: generatedLogLines[i], ClientId: "marko", Hostname: "::1"}
		i += 2
	}
	slog.Info("Done with odd lines")
}

func (suite *OutOfOrderSuite) TestOutOfOrderInsert() {
	t := suite.T()
	start := time.Now()
	expectedCount := int64(10_000)

	logIngestChannel := make(chan db.LogLineWithHost, 1024)

	logServerContext := context.Background()
	defer logServerContext.Done()

	go suite.logServer.Run(logServerContext, logIngestChannel)

	generateOddAndEvenLines(logIngestChannel, expectedCount)

	// Wait for all logs to be inserted into the database
	// The backlog dumps asynchronously, so we need to poll the actual DB count
	timeout := time.After(10 * time.Second)
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

	elapsed := time.Since(start)
	slog.Info("Imported logs", "count", expectedCount, "elapsed", elapsed)

	count := suite.countNumberOfLogsInDb()
	slog.Info("Number of logs in db", "count", count)
	assert.Equal(t, expectedCount, count, "Expected logs in db")

	index := int(expectedCount)
	pageSize := 2_000

	var lastCursorPtr *db.LastCursor
	for {
		logPage, err := suite.logService.GetLogs(context.Background(), db.LogPageRequest{
			ClientId:  "marko",
			Instances: nil,
			PageSize:  int64(pageSize),
			LogLineId: nil,
			Cursor:    lastCursorPtr,
		})
		assert.NoError(t, err)
		lastCursorPtr = validateLogListIsRightOrder(logPage.Logs, index, t)
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
		SequenceNumber: lastLogLine.SequenceNumber,
		Timestamp:      lastLogLine.Timestamp,
		IsBackward:     true,
	}
}
