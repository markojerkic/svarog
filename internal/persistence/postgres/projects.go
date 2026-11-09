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

var (
	// ErrProjectDeleteConfirmationRequired protects projects with retained clients.
	ErrProjectDeleteConfirmationRequired = errors.New("project deletion requires its exact current name")
	// ErrProjectArchiveBusy means an archive build holds the client lock.
	ErrProjectArchiveBusy = errors.New("project archive is in progress")
	// ErrArchiveStoreMissing means archive rows exist but no store can remove them.
	ErrArchiveStoreMissing = errors.New("archive storage is not configured")
	// ErrArchiveStoreMismatch means an archive row belongs to another storage backend.
	ErrArchiveStoreMismatch = errors.New("archive storage backend does not match")
	// ErrArchiveContentCleanup means the project row is gone but a ZIP delete failed.
	ErrArchiveContentCleanup = errors.New("archive content cleanup failed")
)

// StoredArchive is a committed archive object that must be removed after the
// database delete commits. Keys stay internal to the configured backend.
type StoredArchive struct {
	Backend string
	Key     string
}

func CreateProjectWithClients(ctx context.Context, pool *pgxpool.Pool, input ProjectWithClientsInput) (sqlc.Project, error) {
	var project sqlc.Project
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		var err error
		project, err = q.CreateProject(ctx, sqlc.CreateProjectParams{ID: input.ID, Name: input.Name})
		if err != nil {
			return err
		}
		for _, name := range input.Clients {
			if _, err := q.ActivateClientByName(ctx, sqlc.ActivateClientByNameParams{
				ID: uuid.New(), ProjectID: input.ID, Name: name,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return sqlc.Project{}, fmt.Errorf("create project with clients: %w", err)
	}
	return project, nil
}

func UpdateProjectWithClients(ctx context.Context, pool *pgxpool.Pool, input ProjectWithClientsInput) (sqlc.Project, error) {
	var project sqlc.Project
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		var err error

		project, err = q.UpdateProject(ctx, sqlc.UpdateProjectParams{ID: input.ID, Name: input.Name})
		if err != nil {
			return err
		}
		names := input.Clients
		if names == nil {
			names = []string{}
		}
		if err := q.DeactivateUnlistedClients(ctx, sqlc.DeactivateUnlistedClientsParams{ProjectID: input.ID, Names: names}); err != nil {
			return err
		}
		for _, name := range names {
			if _, err := q.ActivateClientByName(ctx, sqlc.ActivateClientByNameParams{
				ID: uuid.New(), ProjectID: input.ID, Name: name,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return sqlc.Project{}, fmt.Errorf("update project with clients: %w", err)
	}
	return project, nil
}

// DeleteProjectWithHistory removes the project, every client, and their logs,
// archive rows and policies. Archive files are deleted only after that commit,
// while the per-client archive locks are still held. A busy lock leaves the
// project unchanged. Projects with clients require their exact current name,
// checked under the project lock so concurrent edits cannot bypass confirmation.
func DeleteProjectWithHistory(ctx context.Context, pool *pgxpool.Pool, projectID uuid.UUID, confirmationName, backend string, cleanup func(context.Context, []StoredArchive) error) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire project delete connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin project delete: %w", err)
	}
	committed := false
	locked := []uuid.UUID{}
	abandoned := false
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if !committed {
			_ = tx.Rollback(cleanupCtx)
		}
		if abandoned {
			return
		}
		session := sqlc.New(conn)
		for _, id := range locked {
			unlocked, unlockErr := session.UnlockArchiveClient(cleanupCtx, id)
			if unlockErr != nil || !unlocked {
				_ = conn.Conn().Close(cleanupCtx)
				return
			}
		}
	}()

	q := sqlc.New(tx)
	project, err := q.LockProject(ctx, projectID)
	if err != nil {
		return err
	}
	clientIDs, err := q.ListProjectClientIDs(ctx, projectID)
	if err != nil {
		return err
	}
	if len(clientIDs) > 0 && confirmationName != project.Name {
		return ErrProjectDeleteConfirmationRequired
	}

	for _, id := range clientIDs {
		ok, err := q.TryLockArchiveClient(ctx, id)
		if err != nil {
			abandoned = true
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = conn.Conn().Close(closeCtx)
			cancel()
			return fmt.Errorf("lock archive client: %w", err)
		}
		if !ok {
			return ErrProjectArchiveBusy
		}
		locked = append(locked, id)
	}

	rows, err := q.ListProjectArchiveObjects(ctx, projectID)
	if err != nil {
		return err
	}
	objects := make([]StoredArchive, len(rows))
	for i, row := range rows {
		if backend == "" {
			return ErrArchiveStoreMissing
		}
		if row.StorageBackend != backend {
			return ErrArchiveStoreMismatch
		}
		objects[i] = StoredArchive{Backend: row.StorageBackend, Key: row.ObjectKey}
	}
	if err := q.DeleteProjectLogs(ctx, projectID); err != nil {
		return err
	}
	if err := q.DeleteProjectArchives(ctx, projectID); err != nil {
		return err
	}
	if err := q.DeleteProjectArchivePolicies(ctx, projectID); err != nil {
		return err
	}
	if err := q.DeleteProjectClients(ctx, projectID); err != nil {
		return err
	}
	if _, err := q.DeleteProject(ctx, projectID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit project delete: %w", err)
	}
	committed = true
	if len(objects) == 0 || cleanup == nil {
		return nil
	}
	if err := cleanup(ctx, objects); err != nil {
		return fmt.Errorf("%w: %w", ErrArchiveContentCleanup, err)
	}
	return nil
}

func ReplaceUserProjects(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, projectIDs []uuid.UUID) error {
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return replaceUserProjects(ctx, sqlc.New(tx), userID, projectIDs)
	})
	if err != nil {
		return fmt.Errorf("replace user projects: %w", err)
	}
	return nil
}

func replaceUserProjects(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, projectIDs []uuid.UUID) error {
	if _, err := q.LockUserForProjectAssignment(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteUserProjects(ctx, userID); err != nil {
		return err
	}
	for _, id := range projectIDs {
		if err := q.AddUserToProject(ctx, sqlc.AddUserToProjectParams{UserID: userID, ProjectID: id}); err != nil {
			return err
		}
	}
	return nil
}
