package db

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/charmbracelet/log"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func generateOddAndEvenLines(logIngestChannel chan<- db.LogLineWithIp, numberOfImportedLogs int64) {
	generatedLogLines := make([]*rpc.LogLine, numberOfImportedLogs)

	for i := 0; i < int(numberOfImportedLogs); i++ {
		generatedLogLines[i] = &rpc.LogLine{
			Message:   fmt.Sprintf("Log line %d", i),
			Timestamp: timestamppb.New(time.Now()),
			Sequence:  int64(i) % math.MaxInt64,
			Client:    "marko",
		}
		if i%2_000 == 0 {
			log.Infof("Generated %d log lines", i)
		}
	}

	i := 0
	for i < int(numberOfImportedLogs) {
		if i%1000 == 0 {
			log.Debugf("Sending even line %d", i)
		}
		logIngestChannel <- db.LogLineWithIp{LogLine: generatedLogLines[i], Ip: "::1"}
		i += 2
	}
	log.Printf("Done with even lines")

	if int(numberOfImportedLogs) != len(generatedLogLines) {
		panic("Expected 1 000 000 logs")
	}

	i = 1
	for i < int(numberOfImportedLogs) {
		if i%1000 == 0 {
			log.Debug("Sending odd line %d", i)
		}
		logIngestChannel <- db.LogLineWithIp{LogLine: generatedLogLines[i], Ip: "::1"}
		i += 2
	}
	log.Printf("Done with odd lines")

}

func (suite *LogsCollectionRepositorySuite) TestOutOfOrderInsert() {
	t := suite.T()
	start := time.Now()

	logIngestChannel := make(chan db.LogLineWithIp, 1024)

	logServerContext := context.Background()
	defer logServerContext.Done()

	go suite.logServer.Run(logServerContext, logIngestChannel)

	generateOddAndEvenLines(logIngestChannel, 10_000)
	for {
		if !suite.logServer.IsBacklogEmpty() {
			log.Info("Backlog still has %d items. Waiting 6s", suite.logServer.BacklogCount())
			time.Sleep(6 * time.Second)
		} else {
			log.Info("Backlog is empty, we can count items", "count", int64(suite.logServer.BacklogCount()))
			break
		}
	}

	suite.logServerContext.Done()

	elapsed := time.Since(start)
	log.Info("Imported %d logs in %s", 10_000, elapsed)

	clients, err := suite.logsRepository.GetClients(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(clients), "Expected one client")

	count := suite.countNumberOfLogsInDb()
	log.Info("Number of logs in db: %d", count)
	assert.Equal(t, int64(10_000), count, "Expected 20 000 logs in db")

	index := int(10_000)
	pageSize := 2_000

	var lastCursorPtr *db.LastCursor
	for {
		logPage, err := suite.logsRepository.GetLogs(context.Background(), "marko", nil, int64(pageSize), nil, lastCursorPtr)
		assert.NoError(t, err)
		lastCursorPtr = validateLogListIsRightOrder(logPage, index, t)
		index -= pageSize
		if index <= 0 || lastCursorPtr == nil {
			break
		}
	}

	assert.LessOrEqual(t, index, 0, "Finished checking logs prematurely")
}
