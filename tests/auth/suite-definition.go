package auth

import (
	"context"
	"log/slog"

	"github.com/gorilla/sessions"
	authlayer "github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuthSuite struct {
	testutils.BaseSuite

	userCollection    *mongo.Collection
	sessionCollection *mongo.Collection

	authService  authlayer.AuthService
	sessionStore sessions.Store
}

// Before all
func (suite *AuthSuite) SetupSuite() {
	config := testutils.DefaultBaseSuiteConfig()
	config.EnableNats = false // Auth tests don't need NATS
	suite.WithConfig(config)

	suite.BaseSuite.SetupSuite()

	suite.userCollection = suite.Collection("users")
	suite.sessionCollection = suite.Collection("sessions")
	suite.sessionStore = authlayer.NewMongoSessionStore(suite.sessionCollection, suite.userCollection, []byte("marko"))

	suite.authService = authlayer.NewMongoAuthService(suite.userCollection, suite.sessionCollection, suite.MongoClient, suite.sessionStore)
}

// After each
func (suite *AuthSuite) TearDownTest() {
	slog.Info("Tearing down test")
	_, err := suite.userCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(suite.T(), err)
	_, err = suite.sessionCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(suite.T(), err)
}

// After all
func (suite *AuthSuite) TearDownSuite() {
	slog.Info("Tearing down suite")
	suite.BaseSuite.TearDownSuite()
}
