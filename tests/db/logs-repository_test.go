package db

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/server/db"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDbSTestSuite struct {
	suite.Suite
	container        *mongodb.MongoDBContainer
	connectionString string
	mongoRepository  *db.MongoLogRepository
	mongoClient      *mongo.Client
	ctx              context.Context
}

func (suite *MongoDbSTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	container, err := mongodb.RunContainer(suite.ctx, testcontainers.WithImage("mongo:6"))
	if err != nil {
		log.Fatalf("Could not start container: %s", err)
	}

	suite.container = container
	suite.connectionString, err = container.ConnectionString(suite.ctx)
	if err != nil {
		log.Fatalf("Could not get connection string: %s", err)
	}

	suite.mongoRepository = db.NewMongoClient(suite.connectionString)
	connectionUrl := suite.connectionString
	suite.mongoClient, err = mongo.Connect(context.Background(), options.Client().ApplyURI(connectionUrl))
	if err != nil {
		log.Fatalf("Could not connect to mongo: %s", err)
	}
}

func (suite *MongoDbSTestSuite) SetupTest() {
	log.Println("Setting up test")
	err := suite.mongoClient.Database("logs").Collection("log_lines").Drop(suite.ctx)
	assert.NoError(suite.T(), err)
}

func (suite *MongoDbSTestSuite) TearDownSuite() {
	if err := suite.container.Terminate(suite.ctx); err != nil {
		log.Fatalf("failed to terminate container: %s", err)
	}
}

func (s *MongoDbSTestSuite) TestNoClients() {
	t := s.T()

	clients, err := s.mongoRepository.GetClients()

	assert.NoError(t, err)

	assert.Equal(t, 0, len(clients))
}

func (s *MongoDbSTestSuite) TestAddClient() {
	t := s.T()

	mockLogLines := []interface{}{
		db.StoredLog{
			Client: db.StoredClient{
				ClientId:  "marko",
				IpAddress: "::1",
			},
			LogLevel:  rpc.LogLevel_INFO,
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		db.StoredLog{
			Client: db.StoredClient{
				ClientId:  "jerkić",
				IpAddress: "::1",
			},
			LogLevel:  rpc.LogLevel_INFO,
			Timestamp: time.Now(),
			LogLine:   "jerkić",
		},
	}

	err := s.mongoRepository.SaveLogs(mockLogLines)
	assert.NoError(t, err)

	clients, err := s.mongoRepository.GetClients()

	assert.NoError(t, err)

	assert.Equal(t, 2, len(clients), fmt.Sprintf("Expected 2 client, got %+v", clients))

	clientIdsSet := make(map[string]bool)
	for _, client := range clients {
		clientIdsSet[client.Client.ClientId] = true
	}

	assert.Equal(t, 2, len(clientIdsSet), fmt.Sprintf("Expected 2 client, got %+v", clientIdsSet))
	assert.Equal(t, true, clientIdsSet["marko"])
	assert.Equal(t, true, clientIdsSet["jerkić"])
}

func TestMongoDbSuite(t *testing.T) {
	suite.Run(t, new(MongoDbSTestSuite))
}
