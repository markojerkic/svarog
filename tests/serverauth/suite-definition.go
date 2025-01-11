package serverauth

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ServerauthSuite struct {
	suite.Suite

	container        *mongodb.MongoDBContainer
	connectionString string

	filesCollection *mongo.Collection

	filesService        files.FileService
	certificatesService serverauth.CertificateService
}

// SetupSuite implements suite.SetupAllSuite.
func (s *ServerauthSuite) SetupSuite() {
	container, err := mongodb.Run(context.Background(), "mongo:7", mongodb.WithReplicaSet("rs0"))
	if err != nil {
		s.T().Fatal(fmt.Sprintf("Could not start container: %s", err))
	}

	s.container = container
	s.connectionString, err = container.ConnectionString(context.Background())
	if err != nil {
		s.T().Fatal(fmt.Sprintf("Could not get connection string: %s", err))
	}

	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	mongoClient, err := mongo.Connect(context.Background(), options.Client().ApplyURI(s.connectionString))
	if err != nil {
		s.T().Fatal(fmt.Sprintf("Could not connect to mongo: %v", err))
	}

	db := mongoClient.Database("svarog")

	s.filesCollection = db.Collection("files")

	s.filesService = files.NewFileService(s.filesCollection)
	s.certificatesService = serverauth.NewCertificateService(s.filesService, mongoClient)

}

// After each
func (s *ServerauthSuite) TearDownSubTest() {
	_, err := s.filesCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(s.T(), err)
}

// After all
func (s *ServerauthSuite) TearDownSuite() {
	err := s.container.Terminate(context.Background())
	if err != nil {
		s.T().Fatal(fmt.Sprintf("Could not terminate container: %s", err))
	}
}

var _ suite.SetupAllSuite = &ServerauthSuite{}
var _ suite.TearDownAllSuite = &ServerauthSuite{}
var _ suite.TearDownSubTest = &ServerauthSuite{}
