package projects

import (
	"context"
	"log/slog"

	authlayer "github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectsSuite struct {
	testutils.BaseSuite

	projectsCollection *mongo.Collection
	userCollection     *mongo.Collection
	sessionCollection  *mongo.Collection
	authService        authlayer.AuthService
}

// SetupSuite implements suite.SetupAllSuite.
func (p *ProjectsSuite) SetupSuite() {
	p.BaseSuite.SetupSuite()

	p.projectsCollection = p.Collection("projects")
	p.userCollection = p.Collection("users")
	p.sessionCollection = p.Collection("sessions")

	sessionStore := authlayer.NewMongoSessionStore(p.sessionCollection, p.userCollection, []byte("test-key"))
	p.authService = authlayer.NewMongoAuthService(p.userCollection, p.sessionCollection, p.MongoClient, sessionStore)
}

// TearDownSubTest implements suite.TearDownSubTest.
func (p *ProjectsSuite) TearDownSubTest() {
	deleteRes, err := p.projectsCollection.DeleteMany(context.Background(), bson.M{})
	slog.Info("Deleted projects", "deleted", deleteRes.DeletedCount)
	assert.NoError(p.T(), err)

	_, err = p.userCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(p.T(), err)

	_, err = p.sessionCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(p.T(), err)
}

// TearDownTest implements suite.TearDownTestSuite.
func (p *ProjectsSuite) TearDownTest() {
	p.TearDownSubTest()
}

// TearDownSuite implements suite.TearDownAllSuite.
func (p *ProjectsSuite) TearDownSuite() {
	p.BaseSuite.TearDownSuite()
}
