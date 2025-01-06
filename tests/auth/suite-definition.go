package auth

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/gorilla/sessions"
	authlayer "github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AuthSuite struct {
	suite.Suite

	container        *mongodb.MongoDBContainer
	connectionString string

	userCollection    *mongo.Collection
	sessionCollection *mongo.Collection

	authService  authlayer.AuthService
	sessionStore sessions.Store
}

// Before all
func (suite *AuthSuite) SetupSuite() {
	container, err := mongodb.Run(context.Background(), "mongo:7", mongodb.WithReplicaSet("rs0"))
	if err != nil {
		suite.T().Fatal(fmt.Sprintf("Could not start container: %s", err))
	}

	suite.container = container
	suite.connectionString, err = container.ConnectionString(context.Background())
	if err != nil {
		suite.T().Fatal(fmt.Sprintf("Could not get connection string: %s", err))
	}

	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	mongoClient, err := mongo.Connect(context.Background(), options.Client().ApplyURI(suite.connectionString))
	if err != nil {
		suite.T().Fatal(fmt.Sprintf("Could not connect to mongo: %v", err))
	}

	db := mongoClient.Database("svarog")
	suite.userCollection = db.Collection("users")
	suite.sessionCollection = db.Collection("sessions")
	suite.sessionStore = authlayer.NewMongoSessionStore(suite.sessionCollection, suite.userCollection, []byte("marko"))

	suite.authService = authlayer.NewMongoAuthService(suite.userCollection, suite.sessionCollection, mongoClient, suite.sessionStore)
}

// After each
func (suite *AuthSuite) TearDownTest() {
	log.Info("Tearing down test")
	_, err := suite.userCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(suite.T(), err)
	_, err = suite.sessionCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(suite.T(), err)
}

// After all
func (suite *AuthSuite) TearDownSuite() {
	log.Info("Tearing down suite")
	if err := suite.container.Terminate(context.Background()); err != nil {
		log.Error("Could not terminate container", "error", err)
	}
}
