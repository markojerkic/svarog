package massimport

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

func TestMassImportSuite(t *testing.T) {
	suite.Run(t, new(MassImportSuite))
}

func generateLogLines(logIngestChannel chan<- db.LogLineWithHost, numberOfImportedLogs int64) {
	for i := 0; i < int(numberOfImportedLogs); i++ {
		logIngestChannel <- db.LogLineWithHost{
			LogLine: &rpc.LogLine{
				Message:   fmt.Sprintf("Log line %d", i),
				Timestamp: time.Now(),
				Sequence:  i,
			},
			ClientId: "marko",
			Hostname: "::1",
		}

		if i%500_000 == 0 {
			slog.Info("Generated log lines", "count", i)
		}
	}
}

var numberOfImportedLogs = int64(1_000)

func (suite *MassImportSuite) TestMassImport() {
	t := suite.T()
	start := time.Now()

	logIngestChannel := make(chan db.LogLineWithHost, 1024)

	logServerContext := context.Background()
	defer logServerContext.Done()

	go suite.logServer.Run(logServerContext, logIngestChannel)
	generateLogLines(logIngestChannel, numberOfImportedLogs)

	for {
		if !suite.logServer.IsBacklogEmpty() {
			slog.Info("Backlog still has items. Waiting 8s", "numItem", suite.logServer.BacklogCount())
			time.Sleep(60 * time.Second)
		} else {
			slog.Info("Backlog is empty, we can count items", "count", int64(suite.logServer.BacklogCount()))
			break
		}
	}

	// Verify logs were saved by counting them
	count := suite.countNumberOfLogsInDb()
	slog.Info("Number of logs in db", "count", count)
	assert.Equal(t, numberOfImportedLogs, count)

	elapsed := time.Since(start)
	slog.Info(fmt.Sprintf("Imported %d logs in %s", numberOfImportedLogs, elapsed))

	// Check all logs if they're in correct order
	index := int(numberOfImportedLogs)
	pageSize := 200_000

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
