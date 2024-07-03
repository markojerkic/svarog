package db

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/markojerkic/svarog/db"
	rpc "github.com/markojerkic/svarog/proto"
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

// Before each, clear the database
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

	client := db.StoredClient{
		ClientId:  "123",
		IpAddress: "::1",
	}

	var mockLogLines = make([]interface{}, 1)
	mockLogLines[0] = db.StoredLog{
		Client:    client,
		LogLevel:  rpc.LogLevel_INFO,
		Timestamp: time.Now(),
		LogLine:   "marko",
	}

	err := s.mongoRepository.SaveLogs(mockLogLines)
	assert.NoError(t, err)

	clients, err := s.mongoRepository.GetClients()

	assert.NoError(t, err)

	assert.Equal(t, 1, len(clients), fmt.Sprintf("Expected 1 client, got %+v", clients))

	assert.Equal(t, client.IpAddress, clients[0].Client.IpAddress)
}

func TestMongoDbSuite(t *testing.T) {
	log.Println("Running test")
	suite.Run(t, new(MongoDbSTestSuite))
}
