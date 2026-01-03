package archive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/util"
	logs "github.com/markojerkic/svarog/internal/server/db"
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
	GetSettings(ctx context.Context, projectID string, clientID string) (types.ArchiveSetting, error)
	UpdateSetting(ctx context.Context, id string, arhiveAfterWeeks int) error
	DeleteSetting(ctx context.Context, id string) error
}

type ArchiveServiceImpl struct {
	mongoClient              *mongo.Client
	archiveCollection        *mongo.Collection
	archiveSettingCollection *mongo.Collection
	filesService             files.FileService
	logsService              logs.LogService
}

// CreateArhiveForClient implements ArhiveService.
func (a *ArchiveServiceImpl) CreateArhiveForClient(ctx context.Context, projectID string, clientID string) (types.ArchiveEntry, error) {

	archive, err := util.StartTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		settings, err := a.GetSettings(ctx, projectID, clientID)
		if err != nil {
			slog.Error("Error getting settings", "error", err)
			return struct{}{}, err
		}

		cuttoffDate := time.Now().Add(-time.Duration(settings.ArhiveAfterWeeks) * 7 * 24 * time.Hour)
		slog.Debug("Creating archive for client", "projectID", projectID, "clientID", "arhiveAfterWeeks", settings.ArhiveAfterWeeks, clientID, "cuttoffDate", cuttoffDate)
		tempDir, err := os.MkdirTemp("", fmt.Sprintf("archive_%s_%s", projectID, clientID))
		if err != nil {
			slog.Error("Error creating temp dir", "error", err)
			return struct{}{}, err
		}
		defer os.RemoveAll(tempDir)
		archiveResult, err := a.createRollingArchive(sc, tempDir, projectID, clientID, cuttoffDate)
		if err != nil {
			slog.Error("Error creating rolling archive", "error", err)
			return struct{}{}, err
		}

		if archiveResult.toDate.IsZero() {
			slog.Debug("No logs found for client", "projectID", projectID, "clientID", clientID)
			return struct{}{}, errors.New("no logs found for period")
		}

		zipFileName := filepath.Base(archiveResult.zipFilePath)
		fileId, err := a.filesService.SaveFile(sc, zipFileName, archiveResult.zipFilePath)
		if err != nil {
			slog.Error("Error saving file", "error", err)
			return struct{}{}, err
		}

		slog.Debug("Successfully created archive for client", "projectID", projectID, "clientID", clientID)

		archive := types.ArchiveEntry{
			FileID:    fileId,
			CreatedAt: primitive.DateTime(time.Now().UnixNano() / int64(time.Millisecond)),
			FromDate:  primitive.DateTime(archiveResult.fromDate.UnixNano() / int64(time.Millisecond)),
			ToDate:    primitive.DateTime(archiveResult.toDate.UnixNano() / int64(time.Millisecond)),
		}

		insertResult, err := a.archiveCollection.InsertOne(sc, archive)
		if err != nil {
			slog.Error("Error inserting archive", "error", err)
			return struct{ archive types.ArchiveEntry }{}, err
		}

		archiveId, ok := insertResult.InsertedID.(primitive.ObjectID)
		if !ok {
			slog.Error("Error converting InsertedID to ObjectID", "error", err)
			return struct{}{}, err
		}

		archive.ID = archiveId

		return struct {
			archive types.ArchiveEntry
		}{archive}, nil

	}, a.mongoClient)

	if err != nil {
		slog.Error("Error creating archive", "error", err)
		return types.ArchiveEntry{}, err
	}

	return archive.(struct{ archive types.ArchiveEntry }).archive, nil

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
		slog.Error("Error converting projectID to ObjectID", "error", err)
		return err
	}

	archiveSetting := types.ArchiveSetting{
		ProjectID:        projectOID,
		ClientID:         clientID,
		ArhiveAfterWeeks: arhiveAfterWeeks,
	}

	_, err = a.archiveSettingCollection.InsertOne(ctx, archiveSetting)
	if err != nil {
		slog.Error("Error inserting archive setting", "error", err)
		return err
	}

	return nil
}

func (a *ArchiveServiceImpl) GetSettings(ctx context.Context, projectID string, clientID string) (types.ArchiveSetting, error) {
	var archiveSetting types.ArchiveSetting
	projectOID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		slog.Error("Error converting projectID to ObjectID", "error", err)
		return archiveSetting, err
	}

	err = a.archiveSettingCollection.FindOne(ctx, bson.M{"project_id": projectOID, "client_id": clientID}).Decode(&archiveSetting)
	if err != nil {
		slog.Error("Error finding archive setting", "error", err)
		return archiveSetting, err
	}

	return archiveSetting, nil
}

// DeleteSetting implements ArhiveService.
// Delete all arhives, files and the setting
func (a *ArchiveServiceImpl) DeleteSetting(ctx context.Context, id string) error {
	_, err := util.StartTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		var archiveSetting types.ArchiveSetting
		archiveSettingID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			slog.Error("Error converting id to ObjectID", "error", err)
			return struct{}{}, err
		}

		err = a.archiveSettingCollection.FindOne(ctx, bson.M{"_id": archiveSettingID}).Decode(&archiveSetting)
		if err != nil {
			slog.Error("Error finding archive setting", "error", err)
			return struct{}{}, err
		}

		_, err = a.archiveSettingCollection.DeleteOne(ctx, bson.M{"_id": archiveSettingID})

		if err != nil {
			slog.Error("Error deleting archive setting", "error", err)
			return struct{}{}, err
		}

		var arhivedFileIDs []primitive.ObjectID
		cursor, err := a.archiveCollection.Find(ctx, bson.M{"projectID": archiveSetting.ProjectID, "clientID": archiveSetting.ClientID})
		if err != nil {
			slog.Error("Error finding archives", "error", err)
			return struct{}{}, err
		}
		for cursor.Next(ctx) {
			var archive types.ArchiveEntry
			err = cursor.Decode(&archive)
			if err != nil {
				slog.Error("Error decoding archive", "error", err)
				return struct{}{}, err
			}
			arhivedFileIDs = append(arhivedFileIDs, archive.FileID)
		}

		for _, id := range arhivedFileIDs {
			err = a.filesService.DeleteFile(ctx, id)
			if err != nil {
				slog.Error("Error deleting file", "error", err)
				return struct{}{}, err
			}
		}

		_, err = a.archiveCollection.DeleteMany(ctx, bson.M{"projectID": archiveSetting.ProjectID, "clientID": archiveSetting.ClientID})
		if err != nil {
			slog.Error("Error deleting archives", "error", err)
			return struct{}{}, err
		}

		return struct{}{}, nil

	}, a.mongoClient)

	if err == nil {
		slog.Debug("Successfully deleted archive setting and all related archives and files")
	}

	return err
}

// UpdateSetting implements ArhiveService.
func (a *ArchiveServiceImpl) UpdateSetting(ctx context.Context, id string, arhiveAfterWeeks int) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		slog.Error("Error converting id to ObjectID", "error", err)
		return err
	}

	_, err = a.archiveSettingCollection.UpdateByID(ctx, oid,
		bson.M{
			"$set": bson.M{
				"arhive_after_weeks": arhiveAfterWeeks,
			},
		})

	if err != nil {
		slog.Error("Error updating archive setting", "error", err)
	}
	return err
}

func (a *ArchiveServiceImpl) createIndexes(ctx context.Context) error {
	// Create compound index for archive settings
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "projectID", Value: 1},
			{Key: "clientID", Value: 1},
		},
		Options: options.Index().
			SetUnique(true).
			SetName("projectID_clientID_unique"),
	}

	_, err := a.archiveSettingCollection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

var _ ArhiveService = &ArchiveServiceImpl{}

func NewArchiveService(mongoClient *mongo.Client,
	archiveCollection *mongo.Collection,
	archiveSettingCollection *mongo.Collection,
	logService logs.LogService,
	filesService files.FileService) *ArchiveServiceImpl {
	if mongoClient == nil {
		panic("mongoClient is nil")
	}
	if archiveCollection == nil {
		panic("archiveCollection is nil")
	}
	if archiveSettingCollection == nil {
		panic("archiveSettingCollection is nil")
	}

	service := &ArchiveServiceImpl{
		mongoClient:              mongoClient,
		archiveCollection:        archiveCollection,
		archiveSettingCollection: archiveSettingCollection,
		filesService:             filesService,
		logsService:              logService,
	}

	service.createIndexes(context.Background())

	return service
}
