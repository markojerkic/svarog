package postgres_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestArchiveListAndOpen(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	project, err := projects.NewPostgresProjectsService(pool).CreateProject(ctx, "ArchiveDownloads", []string{"api", "worker"})
	require.NoError(t, err, "create the download project's clients")
	dir := t.TempDir()
	var service archive.Archiver = archive.NewArchiveService(pool, archive.NewDiskStore(dir))
	scope := archive.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "api")}
	other := archive.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "worker")}
	require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "enable the download client's archive policy")
	ids := make([]uuid.UUID, 0, 2)
	var archiveID uuid.UUID
	var content []byte
	for i := range 2 {
		insertArchiveLog(t, ctx, pool, scope, fmt.Sprintf("page-%d", i))
		result, err := service.Create(ctx, scope)
		require.NoError(t, err, "create archive %d for listing", i)
		require.NotNil(t, result.Archive, "each populated build returns metadata")
		ids = append(ids, result.Archive.ID)
		if i == 0 {
			archiveID = result.Archive.ID
			content = archiveBytes(t, ctx, service, scope, archiveID)
		}
	}
	_, err = pool.Exec(ctx, "UPDATE archives SET created_at = $1 WHERE client_id = $2", time.Now().UTC().Truncate(time.Second), scope.ClientID)
	require.NoError(t, err, "give all archives a tied creation timestamp")
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(b.String(), a.String()) })
	buildingID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO archives (id, client_id, status, storage_backend, object_key, file_name, cutoff_at)
		VALUES ($1, $3, 'building', 'disk', 'building.zip', 'building.zip', now()),
		       ($2, $3, 'deleting', 'disk', 'deleting.zip', 'deleting.zip', now())`, buildingID, uuid.New(), scope.ClientID)
	require.NoError(t, err, "add non-downloadable states that must not appear in the public list")

	t.Run("keyset pagination uses the ID to break timestamp ties", func(t *testing.T) {
		var before *archive.Cursor
		for i, id := range ids {
			page, err := service.List(ctx, scope, archive.ListQuery{Limit: 1, Before: before})
			require.NoError(t, err, "list page %d", i)
			require.Len(t, page.Items, 1, "each page contains one completed archive")
			require.Equal(t, id, page.Items[0].ID, "archives with tied timestamps sort by descending UUID")
			before = page.Next
			if i < len(ids)-1 {
				require.NotNil(t, before, "non-final pages include the next exclusive cursor")
			} else {
				require.Nil(t, before, "the final page has no next cursor")
			}
		}
		page, err := service.List(ctx, other, archive.ListQuery{})
		require.NoError(t, err, "another valid client can list its own archives")
		require.Empty(t, page.Items, "another client's archives are never included")
	})

	t.Run("invalid pagination is rejected", func(t *testing.T) {
		for _, query := range []archive.ListQuery{
			{Limit: -1}, {Limit: 101},
			{Before: &archive.Cursor{ID: uuid.New()}},
			{Before: &archive.Cursor{CreatedAt: time.Now()}},
		} {
			_, err := service.List(ctx, scope, query)
			require.ErrorIs(t, err, archive.ErrInvalidInput, "invalid limits and incomplete cursors are rejected")
		}
	})

	t.Run("downloads are scoped and require completed metadata", func(t *testing.T) {
		for _, id := range []uuid.UUID{uuid.New(), buildingID} {
			_, err := service.Open(ctx, scope, id)
			require.ErrorIs(t, err, archive.ErrArchiveNotFound, "missing and incomplete archives are not downloadable")
		}
		_, err := service.Open(ctx, other, archiveID)
		require.ErrorIs(t, err, archive.ErrArchiveNotFound, "a valid client cannot download another client's archive")
		_, err = service.Open(ctx, scope, uuid.Nil)
		require.ErrorIs(t, err, archive.ErrInvalidInput, "an empty archive ID is invalid input")
	})

	t.Run("disabled and unconfigured policies do not hide metadata", func(t *testing.T) {
		require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{}), "disable future archiving")
		unconfigured := archive.NewArchiveService(pool, nil)
		page, err := unconfigured.List(ctx, scope, archive.ListQuery{})
		require.NoError(t, err, "listing does not require configured storage or an enabled policy")
		require.Len(t, page.Items, len(ids), "disabled clients retain every completed archive")
		require.NotEmpty(t, archiveBytes(t, ctx, service, scope, archiveID), "disabling does not remove completed content")
		_, err = unconfigured.Open(ctx, scope, archiveID)
		require.ErrorIs(t, err, archive.ErrNotConfigured, "opening content requires configured storage")
	})

	t.Run("disk access stays beneath the configured root", func(t *testing.T) {
		var originalKey string
		require.NoError(t, pool.QueryRow(ctx, "SELECT object_key FROM archives WHERE id = $1", archiveID).Scan(&originalKey), "read the path to corrupt as a storage fixture")
		outside := filepath.Join(t.TempDir(), "outside.zip")
		require.NoError(t, os.WriteFile(outside, content, 0o600), "prepare an outside file with otherwise valid content")
		require.NoError(t, os.Symlink(outside, filepath.Join(dir, "escape.zip")), "prepare a symlink escaping the storage root")
		for _, key := range []string{outside, "escape.zip"} {
			_, err := pool.Exec(ctx, "UPDATE archives SET object_key = $1 WHERE id = $2", key, archiveID)
			require.NoError(t, err, "point metadata at an unsafe fixture path")
			_, err = service.Open(ctx, scope, archiveID)
			require.ErrorIs(t, err, archive.ErrArchiveUnavailable, "unsafe paths cannot expose content outside the archive directory")
		}
		_, err := pool.Exec(ctx, "UPDATE archives SET object_key = $1 WHERE id = $2", originalKey, archiveID)
		require.NoError(t, err, "restore the original archive storage key")
		path := filepath.Join(dir, filepath.FromSlash(originalKey))
		require.NoError(t, os.WriteFile(path, []byte("truncated"), 0o600), "truncate a stored archive")
		_, err = service.Open(ctx, scope, archiveID)
		require.ErrorIs(t, err, archive.ErrArchiveUnavailable, "a file with a mismatched size is unavailable")
		require.NoError(t, os.Remove(path), "remove the corrupted fixture")
		_, err = service.Open(ctx, scope, archiveID)
		require.ErrorIs(t, err, archive.ErrArchiveUnavailable, "missing content is distinct from missing metadata")
	})
}
