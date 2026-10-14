package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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

func DeleteArchivePolicy(ctx context.Context, pool *pgxpool.Pool, clientID uuid.UUID) (int64, error) {
	rows, err := sqlc.New(pool).DeleteArchivePolicy(ctx, clientID)
	if err != nil {
		return 0, fmt.Errorf("delete archive policy: %w", err)
	}
	return rows, nil
}
