package db

import (
	"fmt"
	"log"
	"log/slog"
	"math"
	"time"

	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func generateOddAndEvenLines(logIngestChannel chan<- *rpc.LogLine, numberOfImportedLogs int64) {
	generatedLogLines := make([]*rpc.LogLine, numberOfImportedLogs)

	for i := 0; i < int(numberOfImportedLogs); i++ {
		generatedLogLines[i] = &rpc.LogLine{
			Message:   fmt.Sprintf("Log line %d", i),
			Timestamp: timestamppb.New(time.Now()),
			Sequence:  int64(i) % math.MaxInt64,
			Client:    "marko",
		}
		if i%2_000 == 0 {
			log.Printf("Generated %d log lines", i)
		}
	}

	i := 0
	for i < int(numberOfImportedLogs) {
		logIngestChannel <- generatedLogLines[i]
		i += 2
	}
	log.Printf("Done with even lines")

	i = 1
	for i < int(numberOfImportedLogs) {
		logIngestChannel <- generatedLogLines[i]
		i += 2
	}
	log.Printf("Done with odd lines")

}

func (suite *RepositorySuite) TestOutOfOrderInsert() {
	t := suite.T()
	start := time.Now()

	logIngestChannel := make(chan *rpc.LogLine, 1024)

	go suite.logServer.Run(logIngestChannel)

	generateOddAndEvenLines(logIngestChannel, 10_000)
	suite.logServerContext.Done()

	elapsed := time.Since(start)
	slog.Info(fmt.Sprintf("Imported %d logs in %s", 10_000, elapsed))

	clients, err := suite.mongoRepository.GetClients()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(clients), "Expected one client")

	count := suite.countNumberOfLogsInDb()
	slog.Info(fmt.Sprintf("Number of logs in db: %d", count))
	assert.Equal(t, int64(20_000), count, "Expected 20 000 logs in db")

}
