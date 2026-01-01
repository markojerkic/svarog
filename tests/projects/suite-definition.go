package projects

import (
	"context"
	"log/slog"

	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectsSuite struct {
	testutils.BaseSuite

	projectsCollection *mongo.Collection
}

// SetupSuite implements suite.SetupAllSuite.
func (p *ProjectsSuite) SetupSuite() {
	p.BaseSuite.SetupSuite()

	p.projectsCollection = p.Collection("projects")
}

// TearDownSubTest implements suite.TearDownSubTest.
func (p *ProjectsSuite) TearDownSubTest() {
	deleteRes, err := p.projectsCollection.DeleteMany(context.Background(), bson.M{})
	slog.Info("Deleted projects", "deleted", deleteRes.DeletedCount)
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
