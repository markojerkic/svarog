package db

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	ws "github.com/markojerkic/svarog/internal/server/web-socket"
	"github.com/stretchr/testify/assert"
)

func (suite *LogsCollectionRepositorySuite) TestWatchInsert() {
	t := suite.T()
	expectedCount := int64(10)

	logIngestChannel := make(chan db.LogLineWithHost, 1024)

	logServerContext := context.Background()
	defer logServerContext.Done()

	markoSubscription := ws.LogsHub.Subscribe("marko")
	go suite.logServer.Run(logServerContext, logIngestChannel)

	go func() {
		markoUpdates := make([]types.StoredLog, 0, 20)
		timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for {
			isDone := false
			select {
			case log, ok := <-(*markoSubscription).GetUpdates():
				isDone = !ok
				markoUpdates = append(markoUpdates, log)
			case <-timeout.Done():
				isDone = true
			}
			if isDone {
				break
			}
		}

		assert.Equal(t, int(expectedCount), len(markoUpdates), fmt.Sprintf("Got %+v", markoUpdates))
	}()

	generateLogLines(logIngestChannel, expectedCount)

	// Wait for all logs to be inserted into the database
	timeout := time.After(8 * time.Second)
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
			time.Sleep(500 * time.Millisecond)
		}
	}
}
