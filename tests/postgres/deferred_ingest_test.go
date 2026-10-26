package postgres_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/projects"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestMassImport(t *testing.T) {
	const count = 10_000
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	env := newIngestEnv(t, ctx)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lines := make([]db.LogLineWithHost, count)
	for i := range lines {
		lines[i] = ingestLine(env, i, base.Add(time.Duration(i)*time.Millisecond))
	}
	pushThroughAggregator(t, ctx, env, lines)

	got := pageNewestFirst(t, ctx, env, 300)
	require.Len(t, got, count, "every aggregated line is readable")
	requireNewestFirst(t, got)
	for i, log := range got {
		require.Equal(t, fmt.Sprintf("Log line %d", count-1-i), log.LogLine, "newest-first messages have no gaps")
		require.Equal(t, count-1-i, log.SequenceNumber, "each message retains its sequence")
		require.Equal(t, "web", log.Client.InstanceId, "aggregator persists the hostname as the instance")
	}
}

func TestOutOfOrderInsert(t *testing.T) {
	const count = 100_000
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	env := newIngestEnv(t, ctx)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lines := make([]db.LogLineWithHost, 0, count)
	for i := 0; i < count; i += 2 {
		lines = append(lines, ingestLine(env, i, at))
	}
	for i := 1; i < count; i += 2 {
		lines = append(lines, ingestLine(env, i, at))
	}
	pushThroughAggregator(t, ctx, env, lines)

	got := pageNewestFirst(t, ctx, env, 300)
	require.Len(t, got, count, "both arrival waves are stored")
	requireNewestFirst(t, got)
	require.Greater(t, got[0].ID, got[1].ID, "the highest odd sequence was inserted after its even neighbor")
	for i, log := range got {
		require.Equal(t, count-1-i, log.SequenceNumber, "logs follow sequence order rather than arrival order")
		require.Equal(t, fmt.Sprintf("Log line %d", count-1-i), log.LogLine, "out-of-order ingestion preserves each message")
	}
}

func TestFindLoglinePage(t *testing.T) {
	const count = 100_000
	const target = count / 2
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	env := newIngestEnv(t, ctx)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	batch := make([]store.LogInsert, count)
	for i := range batch {
		batch[i] = store.LogInsert{
			ClientID:       env.clientID,
			InstanceName:   "web",
			OccurredAt:     base.Add(time.Duration(i) * time.Millisecond),
			SequenceNumber: int64(i),
			Message:        fmt.Sprintf("Log line %d", i),
		}
	}
	ids, err := store.InsertLogs(ctx, env.pool, batch)
	require.NoError(t, err, "insert %d sequenced lines", count)

	logLine := strconv.FormatInt(ids[target], 10)
	page, err := env.logs.GetLogs(ctx, db.LogPageRequest{
		ProjectId: env.projectID,
		ClientId:  env.clientID.String(),
		PageSize:  300,
		LogLineId: &logLine,
	})
	require.NoError(t, err, "read the page anchored at the middle log")
	require.Len(t, page.Logs, 601, "middle context contains the anchor and 300 lines on each side")
	require.Equal(t, ids[target], page.Logs[300].ID, "logLine anchor is the middle row of the context")
	requireNewestFirst(t, page.Logs)
	for i, log := range page.Logs {
		require.Equal(t, fmt.Sprintf("Log line %d", target+300-i), log.LogLine, "context walks through newer and older neighbors without gaps")
	}
}

func TestArchiveLargeClient(t *testing.T) {
	const oldCount = 100_000
	const recentCount = 10_000
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	env := newIngestEnv(t, ctx)
	dir := t.TempDir()
	var service archive.Archiver = archive.NewArchiveService(env.pool, dir)
	scope := archive.Scope{ProjectID: uuid.MustParse(env.projectID), ClientID: env.clientID}
	require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "create a one-week archive policy")

	recent := time.Now().UTC().Truncate(time.Microsecond)
	old := recent.Add(-14 * 24 * time.Hour)
	batch := make([]store.LogInsert, 0, oldCount+recentCount)
	for i := 0; i < oldCount; i++ {
		batch = append(batch, store.LogInsert{
			ClientID: env.clientID, InstanceName: "web",
			OccurredAt:     old.Add(time.Duration(i) * time.Millisecond),
			SequenceNumber: int64(i), Message: fmt.Sprintf("old %d", i),
		})
	}
	for i := 0; i < recentCount; i++ {
		batch = append(batch, store.LogInsert{
			ClientID: env.clientID, InstanceName: "web",
			OccurredAt:     recent.Add(time.Duration(i) * time.Millisecond),
			SequenceNumber: int64(oldCount + i), Message: fmt.Sprintf("new %d", i),
		})
	}
	_, err := store.InsertLogs(ctx, env.pool, batch)
	require.NoError(t, err, "insert %d old and %d new logs", oldCount, recentCount)

	result, err := service.Create(ctx, scope)
	require.NoError(t, err, "archive twenty full 5000-line pages")
	require.Equal(t, archive.Created, result.Outcome, "a large client returns a completed archive")
	require.NotNil(t, result.Archive, "large archive creation includes metadata")
	record := result.Archive
	require.Equal(t, int64(oldCount), record.LogCount, "only logs older than the cutoff are archived")
	wantZip := make([]string, oldCount)
	for i := range wantZip {
		wantZip[i] = fmt.Sprintf("old %d", oldCount-1-i)
	}
	require.Equal(t, wantZip, zipMessages(t, archiveBytes(t, ctx, service, scope, record.ID)), "zip contains all old lines newest-first across all files")

	wantKept := make([]string, recentCount)
	for i := range wantKept {
		wantKept[i] = fmt.Sprintf("new %d", recentCount-1-i)
	}
	require.Equal(t, wantKept, remainingMessages(t, ctx, env.pool, env.clientID), "all recent logs remain")
	var older int
	err = env.pool.QueryRow(ctx,
		"SELECT count(*) FROM logs WHERE client_id = $1 AND occurred_at < $2",
		env.clientID, record.CutoffAt,
	).Scan(&older)
	require.NoError(t, err, "count logs older than the archive cutoff")
	require.Zero(t, older, "no log older than the cutoff remains")
}

type ingestEnv struct {
	pool      *pgxpool.Pool
	logs      *db.PostgresLogService
	projectID string
	clientID  uuid.UUID
}

func newIngestEnv(t *testing.T, ctx context.Context) ingestEnv {
	t.Helper()
	pool := testutils.NewPostgresPool(t, ctx)
	project, err := projects.NewPostgresProjectsService(pool).CreateProject(ctx, "MassImport", []string{"api"})
	require.NoError(t, err, "create the import project and client")
	return ingestEnv{
		pool: pool, logs: db.NewLogService(pool, nil),
		projectID: project.ID.String(), clientID: clientByName(t, project, "api"),
	}
}

func ingestLine(env ingestEnv, i int, at time.Time) db.LogLineWithHost {
	return db.LogLineWithHost{
		LogLine: &rpc.LogLine{
			Message: fmt.Sprintf("Log line %d", i), Timestamp: at, Sequence: i,
		},
		ProjectId: env.projectID,
		ClientId:  env.clientID.String(),
		Hostname:  "web",
	}
}

func pushThroughAggregator(t *testing.T, ctx context.Context, env ingestEnv, lines []db.LogLineWithHost) {
	t.Helper()
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ch := make(chan db.LogLineWithHost)
	done := make(chan struct{})
	server := db.NewLogServer(env.logs)
	go func() {
		server.Run(runCtx, ch)
		close(done)
	}()
	for _, line := range lines {
		select {
		case ch <- line:
		case <-runCtx.Done():
			close(ch)
			t.Fatal("aggregator must accept all input before the timeout")
		}
	}
	close(ch)
	select {
	case <-done:
	case <-runCtx.Done():
		t.Fatal("aggregator must flush all queued logs before the timeout")
	}
}

func pageNewestFirst(t *testing.T, ctx context.Context, env ingestEnv, pageSize int64) []types.StoredLog {
	t.Helper()
	var cursor *db.LastCursor
	var all []types.StoredLog
	seen := make(map[int64]bool)
	for {
		page, err := env.logs.GetLogs(ctx, db.LogPageRequest{
			ProjectId: env.projectID, ClientId: env.clientID.String(),
			PageSize: pageSize, Cursor: cursor,
		})
		require.NoError(t, err, "read the next newest-first page")
		for _, log := range page.Logs {
			require.False(t, seen[log.ID], "paging must not repeat log ID %d", log.ID)
			seen[log.ID] = true
		}
		all = append(all, page.Logs...)
		if int64(len(page.Logs)) < pageSize {
			return all
		}
		require.NotNil(t, page.BackwardCursor, "a full page provides a cursor for older logs")
		cursor = page.BackwardCursor
	}
}

func requireNewestFirst(t *testing.T, logs []types.StoredLog) {
	t.Helper()
	for i := 1; i < len(logs); i++ {
		prev, cur := logs[i-1], logs[i]
		ordered := prev.Timestamp.After(cur.Timestamp) ||
			(prev.Timestamp.Equal(cur.Timestamp) && prev.SequenceNumber > cur.SequenceNumber) ||
			(prev.Timestamp.Equal(cur.Timestamp) && prev.SequenceNumber == cur.SequenceNumber && prev.ID > cur.ID)
		require.True(t, ordered, "logs at indexes %d and %d must descend by timestamp, sequence, then id", i-1, i)
	}
}
