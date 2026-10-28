package postgres_test

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/projects"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestCreateArchiveForClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool)
	project, err := projectService.CreateProject(ctx, "ArchiveJob", []string{"api", "worker", "write-failure"})
	require.NoError(t, err, "create project and clients for the archive job")
	apiID := clientByName(t, project, "api")
	workerID := clientByName(t, project, "worker")
	dir := t.TempDir()
	var service archive.Archiver = archive.NewArchiveService(pool, archive.NewDiskStore(dir))
	scope := archive.Scope{ProjectID: project.ID, ClientID: apiID}
	old := time.Now().Add(-14 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	recent := time.Now().UTC().Truncate(time.Microsecond)

	require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "create a one-week archive policy")
	_, err = store.InsertLogs(ctx, pool, []store.LogInsert{
		{ClientID: apiID, InstanceName: "web", OccurredAt: old, SequenceNumber: 1, Message: "old-a"},
		{ClientID: apiID, InstanceName: "worker", OccurredAt: old.Add(500 * time.Millisecond), SequenceNumber: 1, Message: "old-worker"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: old.Add(time.Second), SequenceNumber: 2, Message: "old-b"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: recent, SequenceNumber: 3, Message: "recent"},
		{ClientID: workerID, InstanceName: "web", OccurredAt: old, SequenceNumber: 1, Message: "other-client"},
	})
	require.NoError(t, err, "insert logs around the archive cutoff")

	result, err := service.Create(ctx, scope)
	require.NoError(t, err, "archive logs older than the policy cutoff")
	require.Equal(t, archive.Created, result.Outcome, "successful creation returns a completed archive")
	require.NotNil(t, result.Archive, "created results include archive metadata")
	record := result.Archive
	require.Equal(t, int64(3), record.LogCount, "archive counts only the client's old logs")
	require.Equal(t, old, record.FromAt.UTC().Truncate(time.Microsecond), "from_at is the oldest archived log")
	require.Equal(t, old.Add(time.Second), record.ToAt.UTC().Truncate(time.Microsecond), "to_at is the newest archived log")

	download, err := service.Open(ctx, scope, record.ID)
	require.NoError(t, err, "open the completed archive")
	defer download.Content.Close()
	content, err := io.ReadAll(download.Content)
	require.NoError(t, err, "read completed archive content")
	sum, size := sha256.Sum256(content), int64(len(content))
	require.Equal(t, sum, record.SHA256, "returned hash matches the zip")
	require.Equal(t, size, record.SizeBytes, "returned size matches the zip")
	require.Equal(t, *record, download.Archive, "opening an archive returns the persisted creation metadata")
	require.Equal(t, map[string][]string{
		"web":    {"old-b", "old-a"},
		"worker": {"old-worker"},
	}, zipMessagesByInstance(t, content), "each instance keeps its own newest-first files")
	require.False(t, record.CreatedAt.IsZero(), "completed metadata includes its creation timestamp")

	require.Equal(t, []string{"recent"}, remainingMessages(t, ctx, pool, apiID), "recent logs for the archived client remain")
	require.Equal(t, []string{"other-client"}, remainingMessages(t, ctx, pool, workerID), "another client's old logs are not deleted")

	t.Run("no old logs leaves no building row", func(t *testing.T) {
		result, err := service.Create(ctx, scope)
		require.NoError(t, err, "a client with only recent logs is an expected skip")
		require.Equal(t, archive.SkippedNoLogs, result.Outcome, "empty builds return the no-logs outcome")
		require.Nil(t, result.Archive, "empty builds return no archive metadata")
		var building int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM archives WHERE client_id = $1 AND status = 'building'", apiID).Scan(&building), "count building rows after an empty archive")
		require.Zero(t, building, "an empty archive does not leave a building row")
	})

	t.Run("missing directory and missing policy", func(t *testing.T) {
		unconfigured := archive.NewArchiveService(pool, nil)
		_, err := unconfigured.Create(ctx, scope)
		require.ErrorIs(t, err, archive.ErrNotConfigured, "the job refuses to write without ARCHIVE_DIR")
		result, err := service.Create(ctx, archive.Scope{ProjectID: project.ID, ClientID: workerID})
		require.NoError(t, err, "a client without a rule is an expected skip")
		require.Equal(t, archive.SkippedDisabled, result.Outcome, "a client without a rule is disabled")
	})

	t.Run("unfinished build is recovered", func(t *testing.T) {
		buildingID := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO archives (id, client_id, status, storage_backend, object_key, file_name, cutoff_at)
			VALUES ($1, $2, 'building', 'disk', $3, 'busy.zip', now())`,
			buildingID, apiID, apiID.String()+"/busy.zip")
		require.NoError(t, err, "claim the one building slot")
		result, err := service.Create(ctx, scope)
		require.NoError(t, err, "an abandoned building row is recovered before the next build")
		require.Equal(t, archive.SkippedNoLogs, result.Outcome, "recovery does not archive recent logs")
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM archives WHERE id = $1", buildingID).Scan(&count), "count the recovered archive")
		require.Zero(t, count, "recovery removes an abandoned building row")
	})

	t.Run("page boundary tie is archived", func(t *testing.T) {
		tieClient := workerID
		_, err := pool.Exec(ctx, "DELETE FROM logs WHERE client_id = $1", tieClient)
		require.NoError(t, err, "clear the tie client's earlier log")
		tieScope := archive.Scope{ProjectID: project.ID, ClientID: tieClient}
		require.NoError(t, service.SetPolicy(ctx, tieScope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "create a policy for the tie client")
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
		tied, err := service.Create(ctx, tieScope)
		require.NoError(t, err, "archive across a page boundary")
		require.Equal(t, archive.Created, tied.Outcome, "a tied page produces a completed archive")
		require.NotNil(t, tied.Archive, "a tied page returns archive metadata")
		require.Equal(t, int64(5001), tied.Archive.LogCount, "both tied rows are included")
		require.Len(t, zipMessages(t, archiveBytes(t, ctx, service, tieScope, tied.Archive.ID)), 5001, "both tied rows are present in the ZIP")
		require.Empty(t, remainingMessages(t, ctx, pool, tieClient), "every archived tie row is deleted")
	})

	t.Run("write failure preserves logs for retry", func(t *testing.T) {
		failureScope := archive.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "write-failure")}
		failureDir := t.TempDir()
		failureService := archive.NewArchiveService(pool, archive.NewDiskStore(failureDir))
		require.NoError(t, failureService.SetPolicy(ctx, failureScope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "enable the write-failure client's archive policy")
		insertArchiveLog(t, ctx, pool, failureScope, "retained")
		blocked := filepath.Join(failureDir, failureScope.ClientID.String())
		require.NoError(t, os.WriteFile(blocked, []byte("not a directory"), 0o600), "block creation of the client archive directory")
		_, err := failureService.Create(ctx, failureScope)
		require.Error(t, err, "a storage write failure is returned to the caller")
		require.Equal(t, []string{"retained"}, remainingMessages(t, ctx, pool, failureScope.ClientID), "failed writing never deletes source logs")
		require.NoError(t, os.Remove(blocked), "remove the storage obstruction")
		result, err := failureService.Create(ctx, failureScope)
		require.NoError(t, err, "a build can retry after a storage failure")
		require.NotNil(t, result.Archive, "retry creates a completed archive")
	})
}

func TestArchiveConcurrentIngestion(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	pool, service, scope, _ := newArchiveFixture(t, ctx, "ConcurrentArchive")
	insertArchiveLog(t, ctx, pool, scope, "original")
	release := gateArchiveQuery(t, ctx, pool, "logs", "archive_id", "NEW.archive_id IS NOT NULL", 71001)
	results := make(chan archive.CreateResult, 1)
	errors := make(chan error, 1)
	go func() {
		result, err := service.Create(ctx, scope)
		results <- result
		errors <- err
	}()
	waitForArchiveQuery(t, ctx, pool, "ClaimArchiveLogs")
	second, err := service.Create(ctx, scope)
	require.NoError(t, err, "a parallel build is an expected skip")
	require.Equal(t, archive.SkippedBusy, second.Outcome, "only one build may hold the client")
	require.Nil(t, second.Archive, "busy builds return no archive")
	insertArchiveLog(t, ctx, pool, scope, "late")
	require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{}), "changing the policy does not wait for the running build")
	release()
	select {
	case result := <-results:
		require.NoError(t, <-errors, "the original build completes after its gate opens")
		require.Equal(t, archive.Created, result.Outcome, "disabling does not cancel the already started build")
		require.NotNil(t, result.Archive, "the original build returns completed metadata")
		require.Equal(t, []string{"original"}, zipMessages(t, archiveBytes(t, ctx, service, scope, result.Archive.ID)), "only the originally claimed row is archived")
	case <-ctx.Done():
		t.Fatal("the original archive must finish before the test deadline")
	}
	require.Equal(t, []string{"late"}, remainingMessages(t, ctx, pool, scope.ClientID), "a late old log outside the claim snapshot is never deleted")
	require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "re-enable archiving for the remaining late row")
	late, err := service.Create(ctx, scope)
	require.NoError(t, err, "the next build archives the retained late row")
	require.NotNil(t, late.Archive, "the late row produces a completed archive")
	require.Equal(t, []string{"late"}, zipMessages(t, archiveBytes(t, ctx, service, scope, late.Archive.ID)), "the late row is available in the next archive")
}

func TestArchiveInterruptedBuild(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	pool, service, scope, dir := newArchiveFixture(t, ctx, "InterruptedArchive")
	insertArchiveLog(t, ctx, pool, scope, "retained")
	release := gateArchiveQuery(t, ctx, pool, "archives", "status", "NEW.status = 'ready'", 71002)
	buildCtx, cancelBuild := context.WithCancel(ctx)
	defer cancelBuild()
	done := make(chan error, 1)
	go func() {
		_, err := service.Create(buildCtx, scope)
		done <- err
	}()
	waitForArchiveQuery(t, ctx, pool, "MarkArchiveReady")
	cancelBuild()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled, "cancellation remains recognizable through cleanup errors")
	case <-ctx.Done():
		t.Fatal("canceling an archive must stop the worker before the test deadline")
	}
	release()
	require.Equal(t, []string{"retained"}, remainingMessages(t, ctx, pool, scope.ClientID), "an interrupted final transaction never loses source logs")
	// Closing a canceled connection can return before PostgreSQL releases its
	// session lock. Retry only the expected busy outcome during that interval.
	var result archive.CreateResult
	var err error
	require.Eventually(t, func() bool {
		result, err = service.Create(ctx, scope)
		return err != nil || result.Outcome != archive.SkippedBusy
	}, 10*time.Second, 10*time.Millisecond, "the canceled connection eventually releases the archive client lock")
	require.NoError(t, err, "the next build recovers any remaining claim and retries")
	require.Equal(t, archive.Created, result.Outcome, "retrying after cancellation creates the recovered archive")
	require.NotNil(t, result.Archive, "retrying an interrupted build returns a completed archive")
	require.Equal(t, []string{"retained"}, zipMessages(t, archiveBytes(t, ctx, service, scope, result.Archive.ID)), "the retained row is recoverable after cancellation")
	files, err := filepath.Glob(filepath.Join(dir, scope.ClientID.String(), "*"))
	require.NoError(t, err, "list files left after recovery")
	require.Len(t, files, 1, "recovery retains only the completed ZIP and removes unfinished files")
}

func newArchiveFixture(t *testing.T, ctx context.Context, name string) (*pgxpool.Pool, archive.Archiver, archive.Scope, string) {
	t.Helper()
	pool := testutils.NewPostgresPool(t, ctx)
	project, err := projects.NewPostgresProjectsService(pool).CreateProject(ctx, name, []string{"api"})
	require.NoError(t, err, "create the archive fixture's project and client")
	dir := t.TempDir()
	scope := archive.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "api")}
	service := archive.NewArchiveService(pool, archive.NewDiskStore(dir))
	require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "enable the fixture's archive policy")
	return pool, service, scope, dir
}

func insertArchiveLog(t *testing.T, ctx context.Context, pool *pgxpool.Pool, scope archive.Scope, message string) {
	t.Helper()
	_, err := store.InsertLogs(ctx, pool, []store.LogInsert{{
		ClientID: scope.ClientID, InstanceName: "web", OccurredAt: time.Now().UTC().Add(-14 * 24 * time.Hour), SequenceNumber: 1, Message: message,
	}})
	require.NoError(t, err, "insert the old log %q", message)
}

// PostgreSQL triggers provide a deterministic barrier without test-only hooks in
// production code. The test holds the gate until concurrent work is arranged.
func gateArchiveQuery(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, column, condition string, key int64) func() {
	t.Helper()
	gate, err := pool.Acquire(ctx)
	require.NoError(t, err, "acquire a dedicated connection for the test barrier")
	_, err = gate.Exec(ctx, "SELECT pg_advisory_lock($1)", key)
	require.NoError(t, err, "hold the archive test barrier")
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := gate.Exec(cleanupCtx, "SELECT pg_advisory_unlock($1)", key)
		if err != nil {
			_ = gate.Conn().Close(cleanupCtx)
		}
		gate.Release()
		require.NoError(t, err, "release the archive test barrier")
	}
	t.Cleanup(release)
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		CREATE FUNCTION archive_test_gate() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_advisory_xact_lock(%d); RETURN NEW; END $$;
		CREATE TRIGGER archive_test_gate BEFORE UPDATE OF %s ON %s
		FOR EACH ROW WHEN (%s) EXECUTE FUNCTION archive_test_gate();`, key, column, table, condition))
	require.NoError(t, err, "install the deterministic archive barrier")
	return release
}

func waitForArchiveQuery(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM pg_stat_activity WHERE query LIKE $1 AND wait_event = 'advisory'
		)`, "-- name: "+query+"%").Scan(&waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "the archive query %s must reach the deterministic barrier", query)
}

func archiveBytes(t *testing.T, ctx context.Context, service archive.Archiver, scope archive.Scope, id uuid.UUID) []byte {
	t.Helper()
	download, err := service.Open(ctx, scope, id)
	require.NoError(t, err, "open the completed archive through the public interface")
	defer download.Content.Close()
	content, err := io.ReadAll(download.Content)
	require.NoError(t, err, "read completed archive content")
	return content
}

func zipMessages(t *testing.T, content []byte) []string {
	t.Helper()
	grouped := zipMessagesByInstance(t, content)
	var messages []string
	for _, instance := range slices.Sorted(maps.Keys(grouped)) {
		messages = append(messages, grouped[instance]...)
	}
	return messages
}

func zipMessagesByInstance(t *testing.T, content []byte) map[string][]string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err, "open the archive zip")
	files := make(map[string]map[int]*zip.File)
	for _, file := range reader.File {
		require.True(t, filepath.IsLocal(file.Name), "ZIP entries use safe relative paths")
		if file.FileInfo().IsDir() {
			continue
		}
		directory, name := filepath.Split(filepath.FromSlash(file.Name))
		instance := strings.Trim(filepath.ToSlash(directory), "/")
		require.NotEmpty(t, instance, "archive logs are stored in an instance directory")
		require.NotContains(t, instance, "/", "an instance archive has exactly one directory level")
		index, err := strconv.Atoi(strings.TrimSuffix(name[strings.LastIndex(name, "_")+1:], ".log"))
		require.NoError(t, err, "archive log filename ends with a numeric page index")
		if files[instance] == nil {
			files[instance] = make(map[int]*zip.File)
		}
		require.NotContains(t, files[instance], index, "each instance page index occurs only once")
		files[instance][index] = file
	}
	grouped := make(map[string][]string, len(files))
	for instance, pages := range files {
		// ZIP entry order is lexical (_10 before _2); log pages have numeric order.
		for _, index := range slices.Sorted(maps.Keys(pages)) {
			body, err := pages[index].Open()
			require.NoError(t, err, "open a zipped log file")
			scanner := bufio.NewScanner(body)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				marker := "] "
				cut := lastIndex(line, marker)
				require.GreaterOrEqual(t, cut, 0, "every archived line has an instance marker")
				header := line[:cut]
				require.True(t, strings.HasSuffix(header, " "+instance), "an instance file contains only that instance")
				grouped[instance] = append(grouped[instance], line[cut+len(marker):])
			}
			require.NoError(t, scanner.Err(), "read a zipped log file")
			require.NoError(t, body.Close(), "close a zipped log file")
		}
	}
	return grouped
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
