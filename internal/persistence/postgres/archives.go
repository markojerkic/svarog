package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

type ArchivePolicy struct {
	ProjectID         uuid.UUID
	ClientID          uuid.UUID
	ArchiveAfterWeeks int32
	UpdatedAt         time.Time
}

func CreateArchivePolicy(ctx context.Context, pool *pgxpool.Pool, projectID, clientID uuid.UUID, weeks int32) (int64, error) {
	rows, err := sqlc.New(pool).CreateArchivePolicy(ctx, sqlc.CreateArchivePolicyParams{
		ArchiveAfterWeeks: weeks,
		ClientID:          clientID,
		ProjectID:         projectID,
	})
	if err != nil {
		return 0, fmt.Errorf("create archive policy: %w", err)
	}
	return rows, nil
}

func GetArchivePolicy(ctx context.Context, pool *pgxpool.Pool, projectID, clientID uuid.UUID) (ArchivePolicy, error) {
	row, err := sqlc.New(pool).GetArchivePolicy(ctx, sqlc.GetArchivePolicyParams{
		ClientID:  clientID,
		ProjectID: projectID,
	})
	if err != nil {
		return ArchivePolicy{}, err
	}
	return ArchivePolicy{
		ProjectID:         row.ProjectID,
		ClientID:          row.ClientID,
		ArchiveAfterWeeks: row.ArchiveAfterWeeks,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

func UpdateArchivePolicy(ctx context.Context, pool *pgxpool.Pool, clientID uuid.UUID, weeks int32) (int64, error) {
	rows, err := sqlc.New(pool).UpdateArchivePolicy(ctx, sqlc.UpdateArchivePolicyParams{
		ArchiveAfterWeeks: weeks,
		ClientID:          clientID,
	})
	if err != nil {
		return 0, fmt.Errorf("update archive policy: %w", err)
	}
	return rows, nil
}

func InsertBuildingArchive(ctx context.Context, pool *pgxpool.Pool, id, clientID uuid.UUID, objectKey, fileName string, cutoffAt time.Time) error {
	err := sqlc.New(pool).InsertBuildingArchive(ctx, sqlc.InsertBuildingArchiveParams{
		ID:        id,
		ClientID:  clientID,
		ObjectKey: objectKey,
		FileName:  fileName,
		CutoffAt:  cutoffAt,
	})
	if err != nil {
		return fmt.Errorf("insert building archive: %w", err)
	}
	return nil
}

func DeleteBuildingArchive(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	if err := sqlc.New(pool).DeleteBuildingArchive(ctx, id); err != nil {
		return fmt.Errorf("delete building archive: %w", err)
	}
	return nil
}

type ReadyArchive struct {
	ID        uuid.UUID
	ClientID  uuid.UUID
	SizeBytes int64
	SHA256    []byte
	LogCount  int64
	FromAt    time.Time
	ToAt      time.Time
	CutoffAt  time.Time
}

func CompleteArchive(ctx context.Context, pool *pgxpool.Pool, archive ReadyArchive) error {
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		queries := sqlc.New(tx)
		rows, err := queries.MarkArchiveReady(ctx, sqlc.MarkArchiveReadyParams{
			SizeBytes: archive.SizeBytes,
			Sha256:    archive.SHA256,
			LogCount:  archive.LogCount,
			FromAt:    archive.FromAt,
			ToAt:      archive.ToAt,
			ID:        archive.ID,
			ClientID:  archive.ClientID,
		})
		if err != nil {
			return err
		}
		if rows != 1 {
			return fmt.Errorf("archive %s was not building", archive.ID)
		}
		_, err = queries.DeleteClientLogsBefore(ctx, sqlc.DeleteClientLogsBeforeParams{
			ClientID: archive.ClientID,
			CutoffAt: archive.CutoffAt,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("complete archive: %w", err)
	}
	return nil
}

func DeleteArchivePolicy(ctx context.Context, pool *pgxpool.Pool, clientID uuid.UUID) (int64, error) {
	rows, err := sqlc.New(pool).DeleteArchivePolicy(ctx, clientID)
	if err != nil {
		return 0, fmt.Errorf("delete archive policy: %w", err)
	}
	return rows, nil
}
