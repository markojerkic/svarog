package archive

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/files"
	logs "github.com/markojerkic/svarog/internal/server/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ArchiveSuite struct {
	suite.Suite

	container        *mongodb.MongoDBContainer
	connectionString string

	filesCollection          *mongo.Collection
	archiveCollection        *mongo.Collection
	archiveSettingCollection *mongo.Collection
	logCollection            *mongo.Collection

	filesService   files.FileService
	archiveService archive.ArhiveService
	logService     logs.LogService
}

// SetupSuite implements suite.SetupAllSuite.
func (s *ArchiveSuite) SetupSuite() {
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
	s.archiveCollection = db.Collection("archive")
	s.archiveSettingCollection = db.Collection("archive_settings")
	s.logCollection = db.Collection("log_lines")

	s.filesService = files.NewFileService(s.filesCollection)
	s.logService = logs.NewLogService(db)
	s.archiveService = archive.NewArchiveService(mongoClient,
		s.archiveCollection,
		s.archiveSettingCollection,
		s.logService,
		s.filesService)

}

// After each
func (s *ArchiveSuite) TearDownSubTest() {
	log.Info("Cleaning up")
	_, err := s.archiveCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(s.T(), err)
	_, err = s.archiveSettingCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(s.T(), err)
	_, err = s.filesCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(s.T(), err)
}

// TearDownTest implements suite.TearDownTestSuite.
func (a *ArchiveSuite) TearDownTest() {
	a.TearDownSubTest()
}

// After all
func (s *ArchiveSuite) TearDownSuite() {
	err := s.container.Terminate(context.Background())
	if err != nil {
		s.T().Fatal(fmt.Sprintf("Could not terminate container: %s", err))
	}
}

var _ suite.SetupAllSuite = &ArchiveSuite{}
var _ suite.TearDownAllSuite = &ArchiveSuite{}
var _ suite.TearDownSubTest = &ArchiveSuite{}
var _ suite.TearDownTestSuite = &ArchiveSuite{}
