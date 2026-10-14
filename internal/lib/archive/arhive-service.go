package archive

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/util"
	persistence "github.com/markojerkic/svarog/internal/persistence/postgres"
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

var (
	ErrArchivePolicyNotFound = errors.New("archive policy not found")
	ErrArchivePolicyExists   = errors.New("archive policy already exists")
)

type ArchiveServiceImpl struct {
	mongoClient              *mongo.Client
	archiveCollection        *mongo.Collection
	archiveSettingCollection *mongo.Collection
	filesService             files.FileService
	logsService              logs.LogService
	pool                     *pgxpool.Pool
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
	projectUUID, clientUUID, err := parseArchiveScope(projectID, clientID)
	if err != nil {
		return err
	}
	weeks, err := archiveWeeks(arhiveAfterWeeks)
	if err != nil {
		return err
	}
	rows, err := persistence.CreateArchivePolicy(ctx, a.pool, projectUUID, clientUUID, weeks)
	if persistence.IsConstraintViolation(err, persistence.SQLStateUniqueViolation, "archive_policies_pkey") {
		return ErrArchivePolicyExists
	}
	if err != nil {
		slog.Error("Error inserting archive setting", "error", err)
		return err
	}
	if rows == 0 {
		return ErrArchivePolicyNotFound
	}
	return nil
}

func (a *ArchiveServiceImpl) GetSettings(ctx context.Context, projectID string, clientID string) (types.ArchiveSetting, error) {
	projectUUID, clientUUID, err := parseArchiveScope(projectID, clientID)
	if err != nil {
		return types.ArchiveSetting{}, err
	}
	policy, err := persistence.GetArchivePolicy(ctx, a.pool, projectUUID, clientUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return types.ArchiveSetting{}, ErrArchivePolicyNotFound
	}
	if err != nil {
		slog.Error("Error finding archive setting", "error", err)
		return types.ArchiveSetting{}, err
	}
	return archiveSetting(policy), nil
}

// DeleteSetting deletes only the policy. Archive files stay until that segment moves.
func (a *ArchiveServiceImpl) DeleteSetting(ctx context.Context, id string) error {
	clientID, err := parseClientID(id)
	if err != nil {
		return err
	}
	rows, err := persistence.DeleteArchivePolicy(ctx, a.pool, clientID)
	if err != nil {
		slog.Error("Error deleting archive setting", "error", err)
		return err
	}
	if rows == 0 {
		return ErrArchivePolicyNotFound
	}
	return nil
}

// UpdateSetting implements ArhiveService.
func (a *ArchiveServiceImpl) UpdateSetting(ctx context.Context, id string, arhiveAfterWeeks int) error {
	clientID, err := parseClientID(id)
	if err != nil {
		return err
	}
	weeks, err := archiveWeeks(arhiveAfterWeeks)
	if err != nil {
		return err
	}
	rows, err := persistence.UpdateArchivePolicy(ctx, a.pool, clientID, weeks)
	if err != nil {
		slog.Error("Error updating archive setting", "error", err)
		return err
	}
	if rows == 0 {
		return ErrArchivePolicyNotFound
	}
	return nil
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

func parseArchiveScope(projectID, clientID string) (uuid.UUID, uuid.UUID, error) {
	project, err := uuid.Parse(projectID)
	if err != nil || project == uuid.Nil {
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("invalid project id %q", projectID)
	}
	client, err := parseClientID(clientID)
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	return project, client, nil
}

func parseClientID(clientID string) (uuid.UUID, error) {
	client, err := uuid.Parse(clientID)
	if err != nil || client == uuid.Nil {
		return uuid.UUID{}, fmt.Errorf("invalid client id %q", clientID)
	}
	return client, nil
}

func archiveWeeks(weeks int) (int32, error) {
	if weeks <= 0 || weeks > math.MaxInt32 {
		return 0, fmt.Errorf("archive after weeks must be positive")
	}
	return int32(weeks), nil
}

func archiveSetting(policy persistence.ArchivePolicy) types.ArchiveSetting {
	return types.ArchiveSetting{
		ClientID:         policy.ClientID.String(),
		ProjectID:        policy.ProjectID.String(),
		ArhiveAfterWeeks: int(policy.ArchiveAfterWeeks),
		UpdatedAt:        policy.UpdatedAt,
	}
}

var _ ArhiveService = &ArchiveServiceImpl{}

func NewArchiveService(mongoClient *mongo.Client,
	archiveCollection *mongo.Collection,
	archiveSettingCollection *mongo.Collection,
	logService logs.LogService,
	filesService files.FileService,
	pool *pgxpool.Pool) *ArchiveServiceImpl {
	service := &ArchiveServiceImpl{
		filesService: filesService,
		logsService:  logService,
		pool:         pool,
	}
	if mongoClient == nil {
		return service
	}
	if archiveCollection == nil {
		panic("archiveCollection is nil")
	}
	if archiveSettingCollection == nil {
		panic("archiveSettingCollection is nil")
	}
	service.mongoClient = mongoClient
	service.archiveCollection = archiveCollection
	service.archiveSettingCollection = archiveSettingCollection
	service.createIndexes(context.Background())
	return service
}
