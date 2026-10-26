package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

// WithArchiveClientLock holds a session lock and a dedicated connection for the
// entire build, but does not keep a transaction open during file IO. All build
// transactions must use this connection: losing it also loses ownership.
func WithArchiveClientLock(ctx context.Context, pool *pgxpool.Pool, clientID uuid.UUID, fn func(*pgx.Conn) error) (locked bool, err error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("acquire archive connection: %w", err)
	}
	defer conn.Release()
	queries := sqlc.New(conn)
	locked, err = queries.TryLockArchiveClient(ctx, clientID)
	if err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = conn.Conn().Close(closeCtx)
		return false, fmt.Errorf("lock archive client: %w", err)
	}
	if !locked {
		return false, nil
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		unlocked, unlockErr := queries.UnlockArchiveClient(cleanupCtx, clientID)
		if unlockErr != nil || !unlocked {
			// A session lock must never be returned to the pool.
			_ = conn.Conn().Close(cleanupCtx)
			if unlockErr == nil {
				unlockErr = errors.New("archive client lock was not held")
			}
			err = errors.Join(err, fmt.Errorf("unlock archive client: %w", unlockErr))
		}
	}()
	return true, fn(conn.Conn())
}

// BeginArchive freezes the exact cohort in the same transaction as its row.
func BeginArchive(ctx context.Context, conn *pgx.Conn, input sqlc.InsertBuildingArchiveParams) (record sqlc.Archive, count int64, err error) {
	err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		queries := sqlc.New(tx)
		var err error
		record, err = queries.InsertBuildingArchive(ctx, input)
		if err != nil {
			return err
		}
		count, err = queries.ClaimArchiveLogs(ctx, sqlc.ClaimArchiveLogsParams{
			ArchiveID: &input.ID,
			ClientID:  input.ClientID,
			CutoffAt:  input.CutoffAt,
		})
		return err
	})
	if err != nil {
		return sqlc.Archive{}, 0, fmt.Errorf("begin archive: %w", err)
	}
	return record, count, nil
}

type ReadyArchive struct {
	ID        uuid.UUID
	ClientID  uuid.UUID
	SizeBytes int64
	SHA256    []byte
	LogCount  int64
	FromAt    time.Time
	ToAt      time.Time
}

func CompleteArchive(ctx context.Context, conn *pgx.Conn, archive ReadyArchive) error {
	err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
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
		deleted, err := queries.DeleteArchivedLogs(ctx, sqlc.DeleteArchivedLogsParams{
			ClientID:  archive.ClientID,
			ArchiveID: &archive.ID,
		})
		if err != nil {
			return err
		}
		if deleted != archive.LogCount {
			return fmt.Errorf("archive %s contains %d logs but selected %d for deletion", archive.ID, archive.LogCount, deleted)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("complete archive: %w", err)
	}
	return nil
}

// AbortArchive releases claimed logs and removes only a building row. A false
// result (including an already committed ready row) forbids file cleanup.
func AbortArchive(ctx context.Context, conn *pgx.Conn, archiveID, clientID uuid.UUID) (removed bool, err error) {
	err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		queries := sqlc.New(tx)
		if err := queries.ReleaseArchiveLogs(ctx, sqlc.ReleaseArchiveLogsParams{
			ID: archiveID, ClientID: clientID,
		}); err != nil {
			return err
		}
		rows, err := queries.DeleteBuildingArchive(ctx, sqlc.DeleteBuildingArchiveParams{
			ID: archiveID, ClientID: clientID,
		})
		removed = rows == 1
		return err
	})
	if err != nil {
		return false, fmt.Errorf("abort archive: %w", err)
	}
	return removed, nil
}
