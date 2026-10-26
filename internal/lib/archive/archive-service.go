package archive

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

type Service struct {
	pool       *pgxpool.Pool
	archiveDir string
}

var _ Archiver = (*Service)(nil)

func NewArchiveService(pool *pgxpool.Pool, archiveDir string) *Service {
	return &Service{pool: pool, archiveDir: archiveDir}
}

func (a *Service) GetPolicy(ctx context.Context, scope Scope) (Policy, error) {
	if err := validateScope(scope); err != nil {
		return Policy{}, err
	}
	return getPolicy(ctx, a.pool, scope)
}

func (a *Service) SetPolicy(ctx context.Context, scope Scope, policy Policy) error {
	if _, err := a.GetPolicy(ctx, scope); err != nil {
		return err
	}
	queries := sqlc.New(a.pool)
	if !policy.Enabled {
		if err := queries.DeleteArchivePolicy(ctx, sqlc.DeleteArchivePolicyParams{
			ProjectID: scope.ProjectID, ClientID: scope.ClientID,
		}); err != nil {
			return fmt.Errorf("disable archive policy: %w", err)
		}
		return nil
	}
	if err := validatePolicy(policy); err != nil {
		return err
	}
	if a.archiveDir == "" {
		return ErrNotConfigured
	}
	rows, err := queries.SetArchivePolicy(ctx, sqlc.SetArchivePolicyParams{
		ProjectID: scope.ProjectID, ClientID: scope.ClientID,
		ArchiveAfterWeeks: int32(policy.ArchiveAfterWeeks),
	})
	if err != nil {
		return fmt.Errorf("set archive policy: %w", err)
	}
	if rows == 0 {
		return ErrClientNotFound
	}
	return nil
}

func (a *Service) Create(ctx context.Context, scope Scope) (result CreateResult, err error) {
	if _, err := a.GetPolicy(ctx, scope); err != nil {
		return CreateResult{}, err
	}
	locked, err := store.WithArchiveClientLock(ctx, a.pool, scope.ClientID, func(conn *pgx.Conn) error {
		if err := a.recoverBuild(ctx, conn, scope.ClientID); err != nil {
			return err
		}
		policy, err := getPolicy(ctx, conn, scope)
		if err != nil {
			return err
		}
		if !policy.Enabled {
			result.Outcome = SkippedDisabled
			return nil
		}
		if err := validatePolicy(policy); err != nil {
			return err
		}
		if a.archiveDir == "" {
			return ErrNotConfigured
		}
		result, err = a.create(ctx, conn, scope, policy)
		return err
	})
	if err != nil {
		return CreateResult{}, err
	}
	if !locked {
		return CreateResult{Outcome: SkippedBusy}, nil
	}
	return result, nil
}

func (a *Service) List(ctx context.Context, scope Scope, query ListQuery) (Page, error) {
	if err := validateScope(scope); err != nil {
		return Page{}, err
	}
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit < 1 || query.Limit > 100 {
		return Page{}, fmt.Errorf("%w: archive limit must be between 1 and 100", ErrInvalidInput)
	}
	params := sqlc.ListReadyArchivesParams{
		ProjectID: scope.ProjectID, ClientID: scope.ClientID, PageSize: int32(query.Limit + 1),
	}
	if query.Before != nil {
		if query.Before.ID == uuid.Nil || query.Before.CreatedAt.IsZero() {
			return Page{}, fmt.Errorf("%w: archive cursor requires a timestamp and ID", ErrInvalidInput)
		}
		params.HasCursor = true
		params.CursorAt = query.Before.CreatedAt
		params.CursorID = query.Before.ID
	}
	if _, err := getPolicy(ctx, a.pool, scope); err != nil {
		return Page{}, err
	}
	rows, err := sqlc.New(a.pool).ListReadyArchives(ctx, params)
	if err != nil {
		return Page{}, fmt.Errorf("list archives: %w", err)
	}
	page := Page{Items: make([]Archive, 0, min(len(rows), query.Limit))}
	if len(rows) > query.Limit {
		last := rows[query.Limit-1]
		page.Next = &Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		rows = rows[:query.Limit]
	}
	for _, row := range rows {
		page.Items = append(page.Items, publicArchive(row))
	}
	return page, nil
}

func (a *Service) Open(ctx context.Context, scope Scope, archiveID uuid.UUID) (Download, error) {
	if err := validateScope(scope); err != nil {
		return Download{}, err
	}
	if archiveID == uuid.Nil {
		return Download{}, fmt.Errorf("%w: archive ID is required", ErrInvalidInput)
	}
	if _, err := getPolicy(ctx, a.pool, scope); err != nil {
		return Download{}, err
	}
	record, err := sqlc.New(a.pool).GetReadyArchive(ctx, sqlc.GetReadyArchiveParams{
		ProjectID: scope.ProjectID, ClientID: scope.ClientID, ID: archiveID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Download{}, ErrArchiveNotFound
	}
	if err != nil {
		return Download{}, fmt.Errorf("get archive: %w", err)
	}
	if a.archiveDir == "" {
		return Download{}, ErrNotConfigured
	}
	if record.StorageBackend != "disk" {
		return Download{}, ErrArchiveUnavailable
	}
	root, err := os.OpenRoot(a.archiveDir)
	if err != nil {
		return Download{}, fmt.Errorf("%w: open archive directory: %w", ErrArchiveUnavailable, err)
	}
	defer root.Close()
	file, err := root.Open(filepath.FromSlash(record.ObjectKey))
	if err != nil {
		return Download{}, fmt.Errorf("%w: open archive content: %w", ErrArchiveUnavailable, err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != record.SizeBytes.Int64 {
		_ = file.Close()
		return Download{}, fmt.Errorf("%w: invalid archive file", ErrArchiveUnavailable)
	}
	return Download{Archive: publicArchive(record), Content: file}, nil
}

func getPolicy(ctx context.Context, db sqlc.DBTX, scope Scope) (Policy, error) {
	weeks, err := sqlc.New(db).GetArchivePolicy(ctx, sqlc.GetArchivePolicyParams{
		ProjectID: scope.ProjectID, ClientID: scope.ClientID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Policy{}, ErrClientNotFound
	}
	if err != nil {
		return Policy{}, fmt.Errorf("get archive policy: %w", err)
	}
	return Policy{Enabled: weeks > 0, ArchiveAfterWeeks: int(weeks)}, nil
}

func (a *Service) create(ctx context.Context, conn *pgx.Conn, scope Scope, policy Policy) (result CreateResult, err error) {
	id := uuid.New()
	record, count, err := store.BeginArchive(ctx, conn, sqlc.InsertBuildingArchiveParams{
		ID: id, ClientID: scope.ClientID,
		ObjectKey: scope.ClientID.String() + "/" + id.String() + ".zip",
		FileName:  fmt.Sprintf("archive_%s_%s.zip", scope.ClientID, id),
		CutoffAt:  time.Now().UTC().Add(-time.Duration(policy.ArchiveAfterWeeks) * 7 * 24 * time.Hour).Truncate(time.Microsecond),
	})
	if err != nil {
		return CreateResult{}, err
	}
	defer func() {
		if err == nil && result.Outcome == Created {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if cleanupErr := a.abortBuild(cleanupCtx, conn, record); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()
	if count == 0 {
		return CreateResult{Outcome: SkippedNoLogs}, nil
	}
	written, sum, size, err := a.createRollingArchive(ctx, conn, record)
	if err != nil {
		return CreateResult{}, err
	}
	if written.count != count {
		return CreateResult{}, fmt.Errorf("archive %s claimed %d logs but wrote %d", record.ID, count, written.count)
	}
	if err := store.CompleteArchive(ctx, conn, store.ReadyArchive{
		ID: record.ID, ClientID: scope.ClientID,
		SizeBytes: size, SHA256: sum[:], LogCount: written.count,
		FromAt: written.from, ToAt: written.to,
	}); err != nil {
		return CreateResult{}, err
	}
	completed := Archive{
		ID: record.ID, ClientID: scope.ClientID, FileName: record.FileName,
		SizeBytes: size, SHA256: sum, LogCount: written.count,
		CutoffAt: record.CutoffAt, FromAt: written.from, ToAt: written.to,
		CreatedAt: record.CreatedAt,
	}
	return CreateResult{Outcome: Created, Archive: &completed}, nil
}

func (a *Service) recoverBuild(ctx context.Context, conn *pgx.Conn, clientID uuid.UUID) error {
	record, err := sqlc.New(conn).GetBuildingArchive(ctx, clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find unfinished archive: %w", err)
	}
	return a.abortBuild(ctx, conn, record)
}

func (a *Service) abortBuild(ctx context.Context, conn *pgx.Conn, record sqlc.Archive) error {
	removed, err := store.AbortArchive(ctx, conn, record.ID, record.ClientID)
	if err != nil || !removed {
		// Preserve the ZIP when commit state is unknown or already ready.
		return err
	}
	if a.archiveDir == "" || record.StorageBackend != "disk" {
		return nil
	}
	root, err := os.OpenRoot(a.archiveDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open archive directory for cleanup: %w", err)
	}
	defer root.Close()
	for _, name := range []string{record.ObjectKey + ".part", record.ObjectKey} {
		if err := root.Remove(filepath.FromSlash(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("Could not remove unfinished archive file", "archiveID", record.ID, "error", err)
		}
	}
	return nil
}

func publicArchive(record sqlc.Archive) Archive {
	return Archive{
		ID: record.ID, ClientID: record.ClientID, FileName: record.FileName,
		SizeBytes: record.SizeBytes.Int64, SHA256: [32]byte(record.Sha256), LogCount: record.LogCount,
		CutoffAt: record.CutoffAt, FromAt: record.FromAt.Time, ToAt: record.ToAt.Time,
		CreatedAt: record.CreatedAt,
	}
}

func validateScope(scope Scope) error {
	if scope.ProjectID == uuid.Nil || scope.ClientID == uuid.Nil {
		return fmt.Errorf("%w: project and client IDs are required", ErrInvalidInput)
	}
	return nil
}

func validatePolicy(policy Policy) error {
	if policy.Enabled && (policy.ArchiveAfterWeeks < 1 || policy.ArchiveAfterWeeks > MaxArchiveAfterWeeks) {
		return fmt.Errorf("%w: archive weeks must be between 1 and %d", ErrInvalidInput, MaxArchiveAfterWeeks)
	}
	return nil
}
