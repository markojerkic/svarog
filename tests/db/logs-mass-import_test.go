package db

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MassImportTestSuite struct {
	suite.Suite
	container        *mongodb.MongoDBContainer
	connectionString string

	mongoRepository *db.MongoLogRepository
	logServer       db.AggregatingLogServer

	mongoClient *mongo.Client

	testContainerContext context.Context
	logServerContext     context.Context
}

func (suite *MassImportTestSuite) SetupSuite() {
	suite.logServerContext = context.Background()
	suite.testContainerContext = context.Background()

	container, err := mongodb.RunContainer(suite.testContainerContext, testcontainers.WithImage("mongo:6"))
	if err != nil {
		log.Fatalf("Could not start container: %s", err)
	}

	suite.container = container
	suite.connectionString, err = container.ConnectionString(context.Background())
	if err != nil {
		log.Fatalf("Could not get connection string: %s", err)
	}

	suite.mongoRepository = db.NewMongoClient(suite.connectionString)
	suite.logServer = db.NewLogServer(suite.logServerContext, suite.mongoRepository)

	connectionUrl := suite.connectionString
	suite.mongoClient, err = mongo.Connect(context.Background(), options.Client().ApplyURI(connectionUrl))
	if err != nil {
		log.Fatalf("Could not connect to mongo: %s", err)
	}

	logOpts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(os.Stdout, logOpts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func (suite *MassImportTestSuite) TearDownSuite() {
	log.Println("Tearing down suite")
	if err := suite.container.Terminate(suite.testContainerContext); err != nil {
		log.Fatalf("failed to terminate container: %s", err)
	}
}

func TestMassImportSuite(t *testing.T) {
	suite.Run(t, new(MassImportTestSuite))
}

func (suite *MassImportTestSuite) countNumberOfLogsInDb() int64 {
	collection := suite.mongoClient.Database("logs").Collection("log_lines")

	count, err := collection.CountDocuments(context.Background(), bson.D{})
	if err != nil {
		log.Fatalf("Could not count documents: %v", err)
		panic(err)
	}
	return count
}

func generateLogLines(logIngestChannel chan<- *rpc.LogLine, numberOfImportedLogs int64) {
	for i := 0; i < int(numberOfImportedLogs); i++ {
		logIngestChannel <- &rpc.LogLine{
			Message:   fmt.Sprintf("Log line %d", i),
			Timestamp: timestamppb.New(time.Now()),
			Sequence:  int64(i) % 1000,
			Level:     rpc.LogLevel_INFO,
			Client:    "marko",
		}

		if i%500_000 == 0 {
			log.Printf("Generated %d log lines", i)
		}
	}
}

var numberOfImportedLogs = int64(3e6)

func (suite *MassImportTestSuite) TestSaveLogs() {
	t := suite.T()
	start := time.Now()

	logIngestChannel := make(chan *rpc.LogLine, 1024)

	go suite.logServer.Run(logIngestChannel)
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

	clients, err := suite.mongoRepository.GetClients()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(clients))

	count := suite.countNumberOfLogsInDb()
	slog.Info(fmt.Sprintf("Number of logs in db: %d", count))
	assert.Equal(t, numberOfImportedLogs, count)

	elapsed := time.Since(start)
	slog.Info(fmt.Sprintf("Imported %d logs in %s", numberOfImportedLogs, elapsed))
	suite.logServerContext.Done()

	// Check all logs if they're in correct order
	logPage, err := suite.mongoRepository.GetLogs("marko", nil)
	assert.NoError(t, err)
	index := int(numberOfImportedLogs)
	lastCursor := validateLogListIsRightOrder(logPage, index, t)
	index -= 300

	for {
		if index <= 0 {
			break
		}

		logPage, err = suite.mongoRepository.GetLogs("marko", &lastCursor)
		validateLogListIsRightOrder(logPage, index, t)
		index -= 300
	}

}

func validateLogListIsRightOrder(logPage []db.StoredLog, i int, t *testing.T) db.LastCursor {
	for _, log := range logPage {
		if ok := assert.Equal(t, fmt.Sprintf("Log line %d", i-1), log.LogLine); !ok {
			i, _ = strconv.Atoi(regexp.MustCompile(`\d+`).FindString(log.LogLine))
			i++
		}

		i--
	}

	lastLogLine := logPage[len(logPage)-1]

	return db.LastCursor{
		ID:         lastLogLine.ID.Hex(),
		Timestamp:  lastLogLine.Timestamp,
		IsBackward: true,
	}
}
