package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestArchiveSchedulerRunsDueClientsOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	project, err := projects.NewPostgresProjectsService(pool).CreateProject(ctx, "ArchiveScheduler", []string{"api", "worker"})
	require.NoError(t, err, "create the scheduler fixture's project and clients")
	service := archive.NewArchiveService(pool, archive.NewDiskStore(t.TempDir()))
	api := archive.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "api")}
	worker := archive.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "worker")}
	for _, scope := range []archive.Scope{api, worker} {
		require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "enabling a policy makes the client due")
		insertArchiveLog(t, ctx, pool, scope, "old")
	}
	const interval = 24 * time.Hour

	release := holdAdvisoryLock(t, ctx, pool, "svarog:archive:scheduler")
	processed, locked, err := service.RunDue(ctx, interval)
	require.NoError(t, err, "a pass held by another instance is an expected skip")
	require.False(t, locked, "only one instance may run a scheduler pass")
	require.Zero(t, processed, "a skipped pass archives nothing")
	require.Zero(t, readyArchives(t, ctx, pool, api.ClientID), "no archive is created while another instance holds the pass")
	release()

	processed, locked, err = service.RunDue(ctx, interval)
	require.NoError(t, err, "the scheduler pass archives due clients")
	require.True(t, locked, "the pass runs once the scheduler lock is free")
	require.Equal(t, 2, processed, "every due client is processed")
	require.Equal(t, 1, readyArchives(t, ctx, pool, api.ClientID), "the first client gets one archive")
	require.Equal(t, 1, readyArchives(t, ctx, pool, worker.ClientID), "the second client gets one archive")

	processed, _, err = service.RunDue(ctx, interval)
	require.NoError(t, err, "a pass with nothing due succeeds")
	require.Zero(t, processed, "archived clients are not due again before their interval")

	require.NoError(t, service.SetPolicy(ctx, api, archive.Policy{Enabled: true, ArchiveAfterWeeks: 1}), "saving a policy makes the client due again")
	insertArchiveLog(t, ctx, pool, api, "old-while-busy")
	release = holdAdvisoryLock(t, ctx, pool, "svarog:archive:"+api.ClientID.String())
	processed, _, err = service.RunDue(ctx, interval)
	require.NoError(t, err, "a busy client does not fail the pass")
	require.Equal(t, 1, processed, "the due busy client is attempted")
	require.Equal(t, 1, readyArchives(t, ctx, pool, api.ClientID), "a busy client is not archived twice")
	var retriesSoon bool
	require.NoError(t, pool.QueryRow(ctx, "SELECT next_run_at < now() + interval '1 hour' FROM archive_policies WHERE client_id = $1", api.ClientID).Scan(&retriesSoon), "read the busy client's next run")
	require.True(t, retriesSoon, "a busy client is retried well before its daily interval")
	release()
}

func holdAdvisoryLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, key string) func() {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err, "acquire a dedicated connection for the advisory lock")
	_, err = conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1, 0))", key)
	require.NoError(t, err, "hold the advisory lock %q", key)
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := conn.Exec(cleanupCtx, "SELECT pg_advisory_unlock(hashtextextended($1, 0))", key)
		if err != nil {
			_ = conn.Conn().Close(cleanupCtx)
		}
		conn.Release()
		require.NoError(t, err, "release the advisory lock %q", key)
	}
	t.Cleanup(release)
	return release
}

func readyArchives(t *testing.T, ctx context.Context, pool *pgxpool.Pool, clientID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM archives WHERE client_id = $1 AND status = 'ready'", clientID).Scan(&count), "count ready archives")
	return count
}
