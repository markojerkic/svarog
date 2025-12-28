package projects

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ProjectsSuite struct {
	suite.Suite

	container        *mongodb.MongoDBContainer
	connectionString string

	projectsCollection *mongo.Collection

	projectsService projects.ProjectsService
}

// SetupSuite implements suite.SetupAllSuite.
func (p *ProjectsSuite) SetupSuite() {
	container, err := mongodb.Run(context.Background(), "mongo:7", mongodb.WithReplicaSet("rs0"))
	if err != nil {
		p.T().Fatalf("Could not start container: %s", err)
	}

	p.container = container
	p.connectionString, err = container.ConnectionString(context.Background())
	if err != nil {
		p.T().Fatalf("Could not get connection string: %s", err)
	}

	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	mongoClient, err := mongo.Connect(context.Background(), options.Client().ApplyURI(p.connectionString))
	if err != nil {
		p.T().Fatalf("Could not connect to mongo: %v", err)
	}

	db := mongoClient.Database("svarog")

	p.projectsCollection = db.Collection("projects")

	p.projectsService = projects.NewProjectsService(p.projectsCollection, mongoClient)

}

// TearDownSubTest implements suite.TearDownSubTest.
func (p *ProjectsSuite) TearDownSubTest() {
	deleteRes, err := p.projectsCollection.DeleteMany(context.Background(), bson.M{})
	log.Error("Deleted projects", "deleted", deleteRes.DeletedCount)
	assert.NoError(p.T(), err)
}

// TearDownTest implements suite.TearDownTestSuite.
func (p *ProjectsSuite) TearDownTest() {
	p.TearDownSubTest()
}

// TearDownSuite implements suite.TearDownAllSuite.
func (p *ProjectsSuite) TearDownSuite() {
	err := p.container.Terminate(context.Background())
	if err != nil {
		p.T().Fatalf("Could not terminate container: %s", err)
	}
}

var _ suite.SetupAllSuite = &ProjectsSuite{}
var _ suite.TearDownAllSuite = &ProjectsSuite{}
var _ suite.TearDownSubTest = &ProjectsSuite{}
var _ suite.TearDownTestSuite = &ProjectsSuite{}
