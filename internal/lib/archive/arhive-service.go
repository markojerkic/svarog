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
	persistence "github.com/markojerkic/svarog/internal/persistence/postgres"
	logs "github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
)

type ArhiveService interface {
	CreateArhiveForClient(ctx context.Context, projectID string, clientID string) (ArchiveRecord, error)
	DeleteArchiveForClient(ctx context.Context, projectID string, clientID string, olderThanWeeks int) error
	CreateSetting(ctx context.Context, projectID string, clientID string, arhiveAfterWeeks int) error
	GetSettings(ctx context.Context, projectID string, clientID string) (types.ArchiveSetting, error)
	UpdateSetting(ctx context.Context, id string, arhiveAfterWeeks int) error
	DeleteSetting(ctx context.Context, id string) error
}

var (
	ErrArchivePolicyNotFound = errors.New("archive policy not found")
	ErrArchivePolicyExists   = errors.New("archive policy already exists")
	ErrArchiveInProgress     = errors.New("archive already in progress")
)

type ArchiveRecord struct {
	ID             uuid.UUID
	ClientID       uuid.UUID
	Status         string
	StorageBackend string
	ObjectKey      string
	FileName       string
	SizeBytes      int64
	SHA256         [32]byte
	LogCount       int64
	CutoffAt       time.Time
	FromAt         time.Time
	ToAt           time.Time
}

type ArchiveServiceImpl struct {
	logsService logs.LogService
	pool        *pgxpool.Pool
	archiveDir  string
}

// CreateArhiveForClient implements ArhiveService.
func (a *ArchiveServiceImpl) CreateArhiveForClient(ctx context.Context, projectID string, clientID string) (record ArchiveRecord, err error) {
	if a.archiveDir == "" {
		return ArchiveRecord{}, fmt.Errorf("archive directory is not configured")
	}
	_, clientUUID, err := parseArchiveScope(projectID, clientID)
	if err != nil {
		return ArchiveRecord{}, err
	}
	settings, err := a.GetSettings(ctx, projectID, clientID)
	if err != nil {
		return ArchiveRecord{}, err
	}
	cutoff := time.Now().Add(-time.Duration(settings.ArhiveAfterWeeks) * 7 * 24 * time.Hour)
	archiveID := uuid.New()
	objectKey := clientUUID.String() + "/" + archiveID.String() + ".zip"
	fileName := fmt.Sprintf("archive_%s.zip", clientUUID.String())
	if err = persistence.InsertBuildingArchive(ctx, a.pool, archiveID, clientUUID, objectKey, fileName, cutoff); err != nil {
		if persistence.IsConstraintViolation(err, persistence.SQLStateUniqueViolation, "archives_one_building_per_client_idx") {
			return ArchiveRecord{}, ErrArchiveInProgress
		}
		slog.Error("Error inserting archive", "error", err)
		return ArchiveRecord{}, err
	}
	zipPath := filepath.Join(a.archiveDir, filepath.FromSlash(objectKey))
	defer func() {
		if err == nil {
			return
		}
		if removeErr := os.Remove(zipPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			slog.Error("Error removing unfinished archive file", "error", removeErr)
		}
		if deleteErr := persistence.DeleteBuildingArchive(ctx, a.pool, archiveID); deleteErr != nil {
			slog.Error("Error deleting unfinished archive", "error", deleteErr)
		}
	}()
	if err = os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return ArchiveRecord{}, err
	}
	tempDir, err := os.MkdirTemp("", "svarog-archive-")
	if err != nil {
		return ArchiveRecord{}, err
	}
	defer os.RemoveAll(tempDir)
	written, err := a.createRollingArchive(ctx, tempDir, zipPath, projectID, clientID, cutoff)
	if err != nil {
		slog.Error("Error creating rolling archive", "error", err)
		return ArchiveRecord{}, err
	}
	if written.count == 0 {
		return ArchiveRecord{}, errors.New("no logs found for period")
	}
	sum, size, err := hashFile(zipPath)
	if err != nil {
		return ArchiveRecord{}, err
	}
	if err = persistence.CompleteArchive(ctx, a.pool, persistence.ReadyArchive{
		ID:        archiveID,
		ClientID:  clientUUID,
		SizeBytes: size,
		SHA256:    sum[:],
		LogCount:  written.count,
		FromAt:    written.from,
		ToAt:      written.to,
		CutoffAt:  cutoff,
	}); err != nil {
		slog.Error("Error completing archive", "error", err)
		return ArchiveRecord{}, err
	}
	return ArchiveRecord{
		ID:             archiveID,
		ClientID:       clientUUID,
		Status:         "ready",
		StorageBackend: "disk",
		ObjectKey:      objectKey,
		FileName:       fileName,
		SizeBytes:      size,
		SHA256:         sum,
		LogCount:       written.count,
		CutoffAt:       cutoff,
		FromAt:         written.from,
		ToAt:           written.to,
	}, nil
}

// DeleteArchiveForClient implements ArhiveService.
func (a *ArchiveServiceImpl) DeleteArchiveForClient(ctx context.Context, projectID string, clientID string, olderThanWeeks int) error {
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

func NewArchiveService(logService logs.LogService, pool *pgxpool.Pool, archiveDir string) *ArchiveServiceImpl {
	return &ArchiveServiceImpl{
		logsService: logService,
		pool:        pool,
		archiveDir:  archiveDir,
	}
}
