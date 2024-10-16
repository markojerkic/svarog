package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	ws "github.com/markojerkic/svarog/internal/server/web-socket"
	"github.com/stretchr/testify/assert"
)

func (suite *RepositorySuite) TestWatchInsert() {
	t := suite.T()

	logIngestChannel := make(chan db.LogLineWithIp, 1024)

	logServerContext := context.Background()
	defer logServerContext.Done()

	markoSubscription := ws.LogsHub.Subscribe("marko")
	go suite.logServer.Run(logServerContext, logIngestChannel)

	go func() {
		markoUpdates := make([]types.StoredLog, 0, 20)
		timeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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

		assert.Equal(t, 10, len(markoUpdates), fmt.Sprintf("Got %+v", markoUpdates))
	}()

	generateLogLines(logIngestChannel, 10)

	for {
		if !suite.logServer.IsBacklogEmpty() {
			slog.Info(fmt.Sprintf("Backlog still has %d items. Waiting 8s", suite.logServer.BacklogCount()))
			time.Sleep(1 * time.Second)
		} else {
			slog.Info("Backlog is empty, we can count items", slog.Int64("count", int64(suite.logServer.BacklogCount())))
			break
		}
	}
}
