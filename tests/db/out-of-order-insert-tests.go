package db

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log/slog"
)

func generateOddAndEvenLines(logIngestChannel chan<- db.LogLineWithHost, numberOfImportedLogs int64) {
	generatedLogLines := make([]*rpc.LogLine, numberOfImportedLogs)

	for i := 0; i < int(numberOfImportedLogs); i++ {
		generatedLogLines[i] = &rpc.LogLine{
			Message:   fmt.Sprintf("Log line %d", i),
			Timestamp: timestamppb.New(time.Now()),
			Sequence:  int64(i) % math.MaxInt64,
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
		logIngestChannel <- db.LogLineWithHost{LogLine: generatedLogLines[i], Hostname: "::1"}
		i += 2
	}
	slog.Info("Done with even lines")

	if int(numberOfImportedLogs) != len(generatedLogLines) {
		panic("Expected 1 000 000 logs")
	}

	i = 1
	for i < int(numberOfImportedLogs) {
		if i%1000 == 0 {
			slog.Debug("Sending odd line", "index", i)
		}
		logIngestChannel <- db.LogLineWithHost{LogLine: generatedLogLines[i], Hostname: "::1"}
		i += 2
	}
	slog.Info("Done with odd lines")

}

func (suite *LogsCollectionRepositorySuite) TestOutOfOrderInsert() {
	t := suite.T()
	start := time.Now()

	logIngestChannel := make(chan db.LogLineWithHost, 1024)

	logServerContext := context.Background()
	defer logServerContext.Done()

	go suite.logServer.Run(logServerContext, logIngestChannel)

	generateOddAndEvenLines(logIngestChannel, 10_000)
	for {
		if !suite.logServer.IsBacklogEmpty() {
			slog.Info("Backlog still has %d items. Waiting 6s", suite.logServer.BacklogCount())
			time.Sleep(6 * time.Second)
		} else {
			slog.Info("Backlog is empty, we can count items", "count", int64(suite.logServer.BacklogCount()))
			break
		}
	}

	suite.logServerContext.Done()

	elapsed := time.Since(start)
	slog.Info("Imported %d logs in %s", 10_000, elapsed)

	clients, err := suite.logService.GetClients(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(clients), "Expected one client")

	count := suite.countNumberOfLogsInDb()
	slog.Info("Number of logs in db: %d", count)
	assert.Equal(t, int64(10_000), count, "Expected 20 000 logs in db")

	index := int(10_000)
	pageSize := 2_000

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
