package postgres_test

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/projects"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestCreateArchiveForClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool)
	project, err := projectService.CreateProject(ctx, "ArchiveJob", []string{"api", "worker"})
	require.NoError(t, err, "create project and clients for the archive job")
	apiID := clientByName(t, project, "api")
	workerID := clientByName(t, project, "worker")
	dir := t.TempDir()
	logs := db.NewLogService(pool, nil)
	service := archive.NewArchiveService(logs, pool, dir)
	projectID := project.ID.String()
	clientID := apiID.String()
	old := time.Now().Add(-14 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	recent := time.Now().UTC().Truncate(time.Microsecond)

	require.NoError(t, service.CreateSetting(ctx, projectID, clientID, 1), "create a one-week archive policy")
	_, err = store.InsertLogs(ctx, pool, []store.LogInsert{
		{ClientID: apiID, InstanceName: "web", OccurredAt: old, SequenceNumber: 1, Message: "old-a"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: old.Add(time.Second), SequenceNumber: 2, Message: "old-b"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: recent, SequenceNumber: 3, Message: "recent"},
		{ClientID: workerID, InstanceName: "web", OccurredAt: old, SequenceNumber: 1, Message: "other-client"},
	})
	require.NoError(t, err, "insert logs around the archive cutoff")

	record, err := service.CreateArhiveForClient(ctx, projectID, clientID)
	require.NoError(t, err, "archive logs older than the policy cutoff")
	require.Equal(t, "ready", record.Status, "finished archive is ready")
	require.Equal(t, "disk", record.StorageBackend, "archive is stored on disk")
	require.Equal(t, int64(2), record.LogCount, "archive counts only the client's old logs")
	require.Equal(t, old, record.FromAt.UTC().Truncate(time.Microsecond), "from_at is the oldest archived log")
	require.Equal(t, old.Add(time.Second), record.ToAt.UTC().Truncate(time.Microsecond), "to_at is the newest archived log")

	zipPath := filepath.Join(dir, filepath.FromSlash(record.ObjectKey))
	sum, size, err := hashArchive(zipPath)
	require.NoError(t, err, "hash the zip on disk")
	require.Equal(t, sum, record.SHA256, "returned hash matches the zip")
	require.Equal(t, size, record.SizeBytes, "returned size matches the zip")
	require.Equal(t, []string{"old-b", "old-a"}, zipMessages(t, zipPath), "zip contains the archived lines newest first")

	var status string
	var storedSum []byte
	require.NoError(t, pool.QueryRow(ctx, "SELECT status, sha256 FROM archives WHERE id = $1", record.ID).Scan(&status, &storedSum), "read the ready archive row")
	require.Equal(t, "ready", status, "database row is ready")
	require.Equal(t, sum[:], storedSum, "database stores the zip hash")
	require.Equal(t, []string{"recent"}, remainingMessages(t, ctx, pool, apiID), "recent logs for the archived client remain")
	require.Equal(t, []string{"other-client"}, remainingMessages(t, ctx, pool, workerID), "another client's old logs are not deleted")

	t.Run("no old logs leaves no building row", func(t *testing.T) {
		_, err := service.CreateArhiveForClient(ctx, projectID, clientID)
		require.EqualError(t, err, "no logs found for period", "a client with only recent logs is not archived")
		var building int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM archives WHERE client_id = $1 AND status = 'building'", apiID).Scan(&building), "count building rows after an empty archive")
		require.Zero(t, building, "an empty archive does not leave a building row")
	})

	t.Run("missing directory and missing policy", func(t *testing.T) {
		unconfigured := archive.NewArchiveService(logs, pool, "")
		_, err := unconfigured.CreateArhiveForClient(ctx, projectID, clientID)
		require.EqualError(t, err, "archive directory is not configured", "the job refuses to write without ARCHIVE_DIR")
		_, err = service.CreateArhiveForClient(ctx, projectID, workerID.String())
		require.ErrorIs(t, err, archive.ErrArchivePolicyNotFound, "a client without a policy is not archived")
	})

	t.Run("second build is rejected", func(t *testing.T) {
		buildingID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO archives (id, client_id, status, storage_backend, object_key, file_name, cutoff_at)
			VALUES ($1, $2, 'building', 'disk', $3, 'busy.zip', now())`,
			buildingID, apiID, apiID.String()+"/busy.zip")
		require.NoError(t, err, "claim the one building slot")
		_, err = service.CreateArhiveForClient(ctx, projectID, clientID)
		require.ErrorIs(t, err, archive.ErrArchiveInProgress, "a client cannot have two building archives")
		_, err = pool.Exec(ctx, "DELETE FROM archives WHERE id = $1", buildingID)
		require.NoError(t, err, "remove the test building row")
	})

	t.Run("page boundary tie is archived", func(t *testing.T) {
		tieClient := workerID
		_, err := pool.Exec(ctx, "DELETE FROM logs WHERE client_id = $1", tieClient)
		require.NoError(t, err, "clear the tie client's earlier log")
		require.NoError(t, service.CreateSetting(ctx, projectID, tieClient.String(), 1), "create a policy for the tie client")
		batch := make([]store.LogInsert, 0, 5001)
		for i := 1; i <= 4999; i++ {
			batch = append(batch, store.LogInsert{
				ClientID: tieClient, InstanceName: "web", OccurredAt: old.Add(time.Duration(i) * time.Second),
				SequenceNumber: int64(i), Message: "filler",
			})
		}
		batch = append(batch,
			store.LogInsert{ClientID: tieClient, InstanceName: "web", OccurredAt: old, SequenceNumber: 1, Message: "tie-low"},
			store.LogInsert{ClientID: tieClient, InstanceName: "web", OccurredAt: old, SequenceNumber: 1, Message: "tie-high"},
		)
		_, err = store.InsertLogs(ctx, pool, batch)
		require.NoError(t, err, "insert a page of logs with a tied oldest pair")
		tied, err := service.CreateArhiveForClient(ctx, projectID, tieClient.String())
		require.NoError(t, err, "archive across a page boundary")
		require.Equal(t, int64(5001), tied.LogCount, "both tied rows are included")
		require.Empty(t, remainingMessages(t, ctx, pool, tieClient), "every archived tie row is deleted")
	})
}

func hashArchive(path string) ([32]byte, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return [32]byte{}, 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return [32]byte{}, 0, err
	}
	var sum [32]byte
	copy(sum[:], hash.Sum(nil))
	return sum, size, nil
}

func zipMessages(t *testing.T, path string) []string {
	t.Helper()
	reader, err := zip.OpenReader(path)
	require.NoError(t, err, "open the archive zip")
	defer reader.Close()
	var messages []string
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		body, err := file.Open()
		require.NoError(t, err, "open a zipped log file")
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			line := scanner.Text()
			marker := "] "
			if i := len(line) - len(marker); i >= 0 {
				if cut := lastIndex(line, marker); cut >= 0 {
					messages = append(messages, line[cut+len(marker):])
				}
			}
		}
		require.NoError(t, scanner.Err(), "read a zipped log file")
		require.NoError(t, body.Close(), "close a zipped log file")
	}
	return messages
}

func lastIndex(line, marker string) int {
	for i := len(line) - len(marker); i >= 0; i-- {
		if line[i:i+len(marker)] == marker {
			return i
		}
	}
	return -1
}

func remainingMessages(t *testing.T, ctx context.Context, pool *pgxpool.Pool, clientID uuid.UUID) []string {
	t.Helper()
	rows, err := pool.Query(ctx, "SELECT message FROM logs WHERE client_id = $1 ORDER BY occurred_at DESC, sequence_number DESC, id DESC", clientID)
	require.NoError(t, err, "list remaining logs")
	defer rows.Close()
	var messages []string
	for rows.Next() {
		var message string
		require.NoError(t, rows.Scan(&message), "scan a remaining log")
		messages = append(messages, message)
	}
	require.NoError(t, rows.Err(), "read remaining logs")
	return messages
}
