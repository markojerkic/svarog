package archive

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ArhiveService interface {
	CreateArhiveForClient(ctx context.Context, projectID string, clientID string) (types.ArchiveEntry, error)
	GetArhiveFileID(ctx context.Context, projectID string, clientID string) (primitive.ObjectID, error)
	DeleteArchiveForClient(ctx context.Context, projectID string, clientID string, olderThanWeeks int) error
	CreateSetting(ctx context.Context, projectID string, clientID string, arhiveAfterWeeks int) error
	UpdateSetting(ctx context.Context, id string, arhiveAfterWeeks int) error
	DeleteSetting(ctx context.Context, id string) error
}

type ArchiveServiceImpl struct {
	mongoClient              *mongo.Client
	archiveCollection        *mongo.Collection
	archiveSettingCollection *mongo.Collection
	filesService             files.FileService
}

// CreateArhiveForClient implements ArhiveService.
func (a *ArchiveServiceImpl) CreateArhiveForClient(ctx context.Context, projectID string, clientID string) (types.ArchiveEntry, error) {
	panic("unimplemented")
}

// DeleteArchiveForClient implements ArhiveService.
func (a *ArchiveServiceImpl) DeleteArchiveForClient(ctx context.Context, projectID string, clientID string, olderThanWeeks int) error {
	panic("unimplemented")
}

// GetArhiveFileID implements ArhiveService.
func (a *ArchiveServiceImpl) GetArhiveFileID(ctx context.Context, projectID string, clientID string) (primitive.ObjectID, error) {
	panic("unimplemented")
}

// CreateSetting implements ArhiveService.
func (a *ArchiveServiceImpl) CreateSetting(ctx context.Context, projectID string, clientID string, arhiveAfterWeeks int) error {
	projectOID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		log.Error("Error converting projectID to ObjectID", "error", err)
		return err
	}

	archiveSetting := types.ArchiveSetting{
		ProjectID: projectOID,
		ClientID:  clientID,
	}

	_, err = a.archiveSettingCollection.InsertOne(ctx, archiveSetting)
	if err != nil {
		log.Error("Error inserting archive setting", "error", err)
		return err
	}

	return nil
}

// DeleteSetting implements ArhiveService.
// Delete all arhives, files and the setting
func (a *ArchiveServiceImpl) DeleteSetting(ctx context.Context, id string) error {
	_, err := util.StartTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		var archiveSetting types.ArchiveSetting
		archiveSettingID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			log.Error("Error converting id to ObjectID", "error", err)
			return struct{}{}, err
		}

		err = a.archiveSettingCollection.FindOne(ctx, bson.M{"_id": archiveSettingID}).Decode(&archiveSetting)
		if err != nil {
			log.Error("Error finding archive setting", "error", err)
			return struct{}{}, err
		}

		_, err = a.archiveSettingCollection.DeleteOne(ctx, bson.M{"_id": archiveSettingID})

		if err != nil {
			log.Error("Error deleting archive setting", "error", err)
			return struct{}{}, err
		}

		var arhivedFileIDs []primitive.ObjectID
		cursor, err := a.archiveCollection.Find(ctx, bson.M{"projectID": archiveSetting.ProjectID, "clientID": archiveSetting.ClientID})
		if err != nil {
			log.Error("Error finding archives", "error", err)
			return struct{}{}, err
		}
		for cursor.Next(ctx) {
			var archive types.ArchiveEntry
			err = cursor.Decode(&archive)
			if err != nil {
				log.Error("Error decoding archive", "error", err)
				return struct{}{}, err
			}
			arhivedFileIDs = append(arhivedFileIDs, archive.FileID)
		}

		for _, id := range arhivedFileIDs {
			err = a.filesService.DeleteFile(ctx, id)
			if err != nil {
				log.Error("Error deleting file", "error", err)
				return struct{}{}, err
			}
		}

		_, err = a.archiveCollection.DeleteMany(ctx, bson.M{"projectID": archiveSetting.ProjectID, "clientID": archiveSetting.ClientID})
		if err != nil {
			log.Error("Error deleting archives", "error", err)
			return struct{}{}, err
		}

		return struct{}{}, nil

	}, a.mongoClient)

	if err == nil {
		log.Debug("Successfully deleted archive setting and all related archives and files")
	}

	return err
}

// UpdateSetting implements ArhiveService.
func (a *ArchiveServiceImpl) UpdateSetting(ctx context.Context, id string, arhiveAfterWeeks int) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		log.Error("Error converting id to ObjectID", "error", err)
		return err
	}

	_, err = a.archiveSettingCollection.UpdateByID(ctx, oid,
		bson.M{
			"$set": bson.M{
				"arhiveAfterWeeks": arhiveAfterWeeks,
			},
		})

	if err != nil {
		log.Error("Error updating archive setting", "error", err)
	}
	return err
}

func (a *ArchiveServiceImpl) createIndexes() {
	// Create indexes for arhive settings by projectID and clientID, with unique constraint
	_, err := a.archiveSettingCollection.Indexes().CreateOne(context.TODO(), mongo.IndexModel{
		Keys: bson.M{
			"projectID": 1,
			"clientID":  1,
		},
		Options: options.Index().SetUnique(true),
	})

	if err != nil {
		log.Fatal("Error creating index", "err", err)
	}

}

var _ ArhiveService = &ArchiveServiceImpl{}

func NewArchiveService(mongoClient *mongo.Client,
	archiveCollection *mongo.Collection,
	archiveSettingCollection *mongo.Collection,
	filesService files.FileService) *ArchiveServiceImpl {
	if mongoClient == nil {
		log.Fatal("mongoClient is nil")
	}
	if archiveCollection == nil {
		log.Fatal("archiveCollection is nil")
	}
	if archiveSettingCollection == nil {
		log.Fatal("archiveSettingCollection is nil")
	}

	service := &ArchiveServiceImpl{
		mongoClient:       mongoClient,
		archiveCollection: archiveCollection,
		filesService:      filesService,
	}

	service.createIndexes()

	return service
}
