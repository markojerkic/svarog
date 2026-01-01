package archive

import (
	"context"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/files"
	logs "github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ArchiveSuite struct {
	testutils.BaseSuite

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
	config := testutils.DefaultBaseSuiteConfig()
	config.EnableNats = false // Archive tests don't need NATS
	s.WithConfig(config)

	s.BaseSuite.SetupSuite()

	s.filesCollection = s.Collection("files")
	s.archiveCollection = s.Collection("archive")
	s.archiveSettingCollection = s.Collection("archive_settings")
	s.logCollection = s.Collection("log_lines")

	s.filesService = files.NewFileService(s.filesCollection)
	s.logService = logs.NewLogService(s.Database)
	s.archiveService = archive.NewArchiveService(s.MongoClient,
		s.archiveCollection,
		s.archiveSettingCollection,
		s.logService,
		s.filesService)
}

// After each
func (s *ArchiveSuite) TearDownSubTest() {
	_, err := s.logCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(s.T(), err)
	_, err = s.archiveCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(s.T(), err)
	_, err = s.archiveSettingCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(s.T(), err)
	_, err = s.filesCollection.DeleteMany(context.Background(), bson.M{})
	assert.NoError(s.T(), err)
}

// TearDownTest implements suite.TearDownTestSuite.
func (s *ArchiveSuite) TearDownTest() {
	s.TearDownSubTest()
}

// After all
func (s *ArchiveSuite) TearDownSuite() {
	s.BaseSuite.TearDownSuite()
}
