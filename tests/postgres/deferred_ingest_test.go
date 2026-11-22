package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestMassImport(t *testing.T) {
	const count = 10_000
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	env := newIngestEnv(t, ctx)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lines := make([]logstore.Event, count)
	for i := range lines {
		lines[i] = ingestLine(env, i, base.Add(time.Duration(i)*time.Millisecond))
	}
	commitIngestLines(t, ctx, env, lines)

	got := pageNewestFirst(t, ctx, env, 300)
	require.Len(t, got, count, "every committed line is readable")
	requireNewestFirst(t, got)
	for i, log := range got {
		require.Equal(t, fmt.Sprintf("Log line %d", count-1-i), log.Message, "newest-first messages have no gaps")
		require.Equal(t, int64(count-1-i), log.Sequence, "each message retains its sequence")
		require.Equal(t, "web", log.Instance, "ingest persists the hostname as the instance")
	}
}

func TestOutOfOrderInsert(t *testing.T) {
	const count = 100_000
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	env := newIngestEnv(t, ctx)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lines := make([]logstore.Event, 0, count)
	for i := 0; i < count; i += 2 {
		lines = append(lines, ingestLine(env, i, at))
	}
	for i := 1; i < count; i += 2 {
		lines = append(lines, ingestLine(env, i, at))
	}
	commitIngestLines(t, ctx, env, lines)

	got := pageNewestFirst(t, ctx, env, 300)
	require.Len(t, got, count, "both arrival waves are stored")
	requireNewestFirst(t, got)
	require.Greater(t, got[0].ID, got[1].ID, "the highest odd sequence was inserted after its even neighbor")
	for i, log := range got {
		require.Equal(t, int64(count-1-i), log.Sequence, "logs follow sequence order rather than arrival order")
		require.Equal(t, fmt.Sprintf("Log line %d", count-1-i), log.Message, "out-of-order ingestion preserves each message")
	}
}

func TestFindLoglinePage(t *testing.T) {
	const count = 100_000
	const target = count / 2
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	env := newIngestEnv(t, ctx)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	batch := make([]logstore.Row, count)
	for i := range batch {
		batch[i] = logstore.Row{
			Scope: logstore.Scope{ClientID: env.scope.ClientID}, Instance: "web",
			OccurredAt: base.Add(time.Duration(i) * time.Millisecond),
			Sequence:   int64(i),
			Message:    fmt.Sprintf("Log line %d", i),
		}
	}
	ids, err := testutils.InsertLogFixtures(ctx, env.pool, batch)
	require.NoError(t, err, "insert %d sequenced lines", count)

	page, found, err := env.logs.Around(ctx, env.scope, ids[target], 300, nil)
	require.NoError(t, err, "read the page anchored at the middle log")
	require.True(t, found, "the middle log must be an authorized anchor")
	require.Len(t, page.Rows, 601, "middle context contains the anchor and 300 lines on each side")
	require.Equal(t, ids[target], page.Rows[300].ID, "logLine anchor is the middle row of the context")
	requireNewestFirst(t, page.Rows)
	for i, log := range page.Rows {
		require.Equal(t, fmt.Sprintf("Log line %d", target+300-i), log.Message, "context walks through newer and older neighbors without gaps")
	}
}

func TestArchiveLargeClient(t *testing.T) {
	const oldCount = 100_000
	const recentCount = 10_000
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	env := newIngestEnv(t, ctx)
	dir := t.TempDir()
	var service archive.Archiver = archive.NewArchiveService(env.pool, archive.NewDiskStore(dir), "")
	scope := archive.Scope{ProjectID: env.scope.ProjectID, ClientID: env.scope.ClientID}
	require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "create a one-week archive policy")

	recent := time.Now().UTC().Truncate(time.Microsecond)
	old := recent.Add(-14 * 24 * time.Hour)
	batch := make([]logstore.Row, 0, oldCount+recentCount)
	for i := 0; i < oldCount; i++ {
		batch = append(batch, logstore.Row{
			Scope: logstore.Scope{ClientID: env.scope.ClientID}, Instance: "web",
			OccurredAt: old.Add(time.Duration(i) * time.Millisecond),
			Sequence:   int64(i), Message: fmt.Sprintf("old %d", i),
		})
	}
	for i := 0; i < recentCount; i++ {
		batch = append(batch, logstore.Row{
			Scope: logstore.Scope{ClientID: env.scope.ClientID}, Instance: "web",
			OccurredAt: recent.Add(time.Duration(i) * time.Millisecond),
			Sequence:   int64(oldCount + i), Message: fmt.Sprintf("new %d", i),
		})
	}
	_, err := testutils.InsertLogFixtures(ctx, env.pool, batch)
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
	require.Equal(t, wantKept, remainingMessages(t, ctx, env.pool, env.scope.ClientID), "all recent logs remain")
	var older int
	err = env.pool.QueryRow(ctx,
		"SELECT count(*) FROM logs WHERE client_id = $1 AND occurred_at < $2",
		env.scope.ClientID, record.CutoffAt,
	).Scan(&older)
	require.NoError(t, err, "count logs older than the archive cutoff")
	require.Zero(t, older, "no log older than the cutoff remains")
}

type ingestEnv struct {
	pool  *pgxpool.Pool
	logs  *logstore.Store
	scope logstore.Scope
}

func newIngestEnv(t *testing.T, ctx context.Context) ingestEnv {
	t.Helper()
	pool := testutils.NewPostgresPool(t, ctx)
	project, err := projects.NewPostgresProjectsService(pool, nil).Create(ctx, projects.ProjectInput{Name: "MassImport", Clients: []string{"api"}})
	require.NoError(t, err, "create the import project and client")
	return ingestEnv{
		pool: pool, logs: logstore.New(pool),
		scope: logstore.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "api")},
	}
}

func ingestLine(env ingestEnv, i int, at time.Time) logstore.Event {
	return logstore.Event{
		Key:      logevent.MessageKey("logs."+env.scope.ProjectID.String()+"."+env.scope.ClientID.String(), []byte(fmt.Sprintf("fixture:%d:%s", i, at.Format(time.RFC3339Nano)))),
		Scope:    env.scope,
		Instance: "web", OccurredAt: at, Sequence: int64(i), Message: fmt.Sprintf("Log line %d", i),
	}
}

func commitIngestLines(t *testing.T, ctx context.Context, env ingestEnv, lines []logstore.Event) {
	t.Helper()
	for offset := 0; offset < len(lines); offset += 1000 {
		_, err := env.logs.Commit(ctx, lines[offset:min(offset+1000, len(lines))])
		require.NoError(t, err, "commit all lines through the ingest writer")
	}
}

func pageNewestFirst(t *testing.T, ctx context.Context, env ingestEnv, pageSize int) []logstore.Row {
	t.Helper()
	var cursor *logstore.Cursor
	var all []logstore.Row
	seen := make(map[int64]bool)
	for {
		page, err := env.logs.Page(ctx, logstore.PageQuery{
			Scope: env.scope, Limit: pageSize, Older: cursor,
		})
		require.NoError(t, err, "read the next newest-first page")
		for _, log := range page.Rows {
			require.False(t, seen[log.ID], "paging must not repeat log ID %d", log.ID)
			seen[log.ID] = true
		}
		all = append(all, page.Rows...)
		if len(page.Rows) < pageSize {
			return all
		}
		require.NotNil(t, page.Older, "a full page provides a cursor for older logs")
		cursor = page.Older
	}
}

func requireNewestFirst(t *testing.T, logs []logstore.Row) {
	t.Helper()
	for i := 1; i < len(logs); i++ {
		prev, cur := logs[i-1], logs[i]
		ordered := prev.OccurredAt.After(cur.OccurredAt) ||
			(prev.OccurredAt.Equal(cur.OccurredAt) && prev.Sequence > cur.Sequence) ||
			(prev.OccurredAt.Equal(cur.OccurredAt) && prev.Sequence == cur.Sequence && prev.ID > cur.ID)
		require.True(t, ordered, "logs at indexes %d and %d must descend by timestamp, sequence, then id", i-1, i)
	}
}
