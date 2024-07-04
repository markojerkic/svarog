package db

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math"
	"os"
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
)

type MassImportTestSuite struct {
	suite.Suite
	container        *mongodb.MongoDBContainer
	connectionString string

	mongoRepository *db.MongoLogRepository
	logServer       db.AggregatingLogServer

	mongoClient *mongo.Client
	ctx         context.Context
}

func (suite *MassImportTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	container, err := mongodb.RunContainer(context.Background(), testcontainers.WithImage("mongo:6"))
	if err != nil {
		log.Fatalf("Could not start container: %s", err)
	}

	suite.container = container
	suite.connectionString, err = container.ConnectionString(suite.ctx)
	if err != nil {
		log.Fatalf("Could not get connection string: %s", err)
	}

	suite.mongoRepository = db.NewMongoClient(suite.connectionString)
	suite.logServer = db.NewLogServer(context.Background(), suite.mongoRepository)

	connectionUrl := suite.connectionString
	suite.mongoClient, err = mongo.Connect(suite.ctx, options.Client().ApplyURI(connectionUrl))
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

func (suite *MassImportTestSuite) SetupTest() {
	log.Println("Setting up test")
	err := suite.mongoClient.Database("logs").Collection("log_lines").Drop(suite.ctx)
	assert.NoError(suite.T(), err)
}

func (suite *MassImportTestSuite) TearDownSuite() {
	if err := suite.container.Terminate(suite.ctx); err != nil {
		log.Fatalf("failed to terminate container: %s", err)
	}
}

func TestMassImportSuite(t *testing.T) {
	suite.Run(t, new(MassImportTestSuite))
}

func generateLogLines(logIngestChannel chan<- *rpc.LogLine, numberOfImportedLogs int64) {

	for i := 0; i < int(numberOfImportedLogs); i++ {
		logIngestChannel <- &rpc.LogLine{
			Message: fmt.Sprintf("Log line %d", i),
			Level:   rpc.LogLevel_INFO,
			Client:  "marko",
		}

		if i%100_000 == 0 {
			log.Printf("Generated %d log lines", i)
		}
	}
}

func (suite *MassImportTestSuite) countNumberOfLogsInDb() int64 {
	count, err := suite.mongoClient.Database("logs").Collection("log_lines").CountDocuments(suite.ctx, bson.D{})

	if err != nil {
		log.Fatalf("Could not count documents: %v", err)
	}

	return count
}

var numberOfImportedLogs = int64(3e6)

func (suite *MassImportTestSuite) TestSaveLogs() {
	t := suite.T()

	logIngestChannel := make(chan *rpc.LogLine, 10*1024*1024)

	go suite.logServer.Run(logIngestChannel)
	generateLogLines(logIngestChannel, numberOfImportedLogs)
	suite.ctx.Done()

	for {
		if !suite.logServer.IsBacklogEmpty() {
			slog.Info(fmt.Sprintf("Backlog still has %d items. Waiting 5s", suite.logServer.BacklogCount()))
			<-time.After(5 * time.Second)
		} else {
			slog.Info("Backlog is empty, we can count items", slog.Int64("count", int64(suite.logServer.BacklogCount())))
			break
		}
	}

	count := suite.countNumberOfLogsInDb()
	assert.Equal(t, math.MaxInt64, count)
}
