package websocket

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestWebSocketOrderingSuite(t *testing.T) {
	suite.Run(t, new(WebSocketOrderingSuite))
}

func (suite *WebSocketOrderingSuite) TestWebSocketOrdering() {
	t := suite.T()
	start := time.Now()

	projectId := "test-project"
	clientId := "test-client"
	expectedCount := int64(200)

	// Subscribe to WebSocket before ingesting logs
	lines := make(chan []byte, 1000)
	subscription, err := suite.WatchHub.Subscribe(projectId, clientId, lines)
	assert.NoError(t, err, "Should subscribe to WebSocket")
	defer subscription.Unsubscribe()

	// Create channel for log ingestion
	logIngestChannel := make(chan db.LogLineWithHost, 1024)

	// Start log server
	go suite.logServer.Run(suite.logServerContext, logIngestChannel)

	// Ingest logs sequentially with sequential numbers
	for i := 0; i < int(expectedCount); i++ {
		logIngestChannel <- db.LogLineWithHost{
			LogLine: &rpc.LogLine{
				Message:   fmt.Sprintf("Log line %d", i),
				Timestamp: time.Now(),
				Sequence:  i,
			},
			ClientId:  clientId,
			ProjectId: projectId,
			Hostname:  "::1",
		}
	}

	slog.Info("Finished sending logs to ingest channel")

	// Collect all logs from WebSocket
	receivedLogs := make([]int, 0, expectedCount)
	timeout := time.After(30 * time.Second)

	// Pattern to extract the log number from rendered HTML
	// The HTML should contain something like "Log line 123"
	logNumberPattern := regexp.MustCompile(`Log line (\d+)`)

collectLoop:
	for {
		select {
		case line := <-lines:
			// Extract the log number from the HTML
			matches := logNumberPattern.FindSubmatch(line)
			if len(matches) >= 2 {
				logNum, err := strconv.Atoi(string(matches[1]))
				if err == nil {
					receivedLogs = append(receivedLogs, logNum)
					if len(receivedLogs)%50 == 0 {
						slog.Info("Received logs via WebSocket", "count", len(receivedLogs))
					}
				}
			}

			// Check if we've received all logs
			if len(receivedLogs) >= int(expectedCount) {
				break collectLoop
			}

		case <-timeout:
			t.Fatalf("Timeout waiting for WebSocket messages. Expected %d, got %d", expectedCount, len(receivedLogs))
		}
	}

	elapsed := time.Since(start)
	slog.Info("Received all logs via WebSocket", "count", len(receivedLogs), "elapsed", elapsed)

	// Verify that logs were received in order
	assert.Equal(t, int(expectedCount), len(receivedLogs), "Should receive all logs")

	for i := 0; i < len(receivedLogs); i++ {
		if !assert.Equal(t, i, receivedLogs[i], fmt.Sprintf("Log at position %d should have number %d", i, i)) {
			// Print surrounding logs for context
			start := i - 5
			if start < 0 {
				start = 0
			}
			end := i + 5
			if end > len(receivedLogs) {
				end = len(receivedLogs)
			}
			slog.Error("Ordering error context", "received", receivedLogs[start:end], "position", i)
			t.FailNow()
		}
	}

	slog.Info("All logs received in correct order!")
}
