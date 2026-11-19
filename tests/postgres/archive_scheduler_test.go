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
	project, err := projects.NewPostgresProjectsService(pool, nil).Create(ctx, projects.ProjectInput{Name: "ArchiveScheduler", Clients: []string{"api", "worker", "idle"}})
	require.NoError(t, err, "create the scheduler fixture's project and clients")
	service := archive.NewArchiveService(pool, archive.NewDiskStore(t.TempDir()))
	api := archive.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "api")}
	worker := archive.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "worker")}
	idle := archive.Scope{ProjectID: project.ID, ClientID: clientByName(t, project, "idle")}
	scopes := []archive.Scope{api, worker, idle}
	for i, scope := range scopes {
		require.NoError(t, service.SetPolicy(ctx, scope, archive.Policy{Enabled: true, ArchiveAfterWeeks: i + 1}), "enable each client's own archive interval")
	}
	insertArchiveLog(t, ctx, pool, api, "old-api")
	insertArchiveLog(t, ctx, pool, worker, "old-worker")

	processed, _, err := service.RunDue(ctx)
	require.NoError(t, err, "a pass before the first policy interval succeeds")
	require.Zero(t, processed, "enabling policies does not immediately archive old logs")
	_, err = pool.Exec(ctx, "UPDATE archive_policies SET next_run_at = now() - interval '1 second' WHERE client_id IN ($1, $2, $3)", api.ClientID, worker.ClientID, idle.ClientID)
	require.NoError(t, err, "advance all three policies to their first automatic run")

	release := holdAdvisoryLock(t, ctx, pool, "svarog:archive:scheduler")
	processed, locked, err := service.RunDue(ctx)
	require.NoError(t, err, "a pass held by another instance is an expected skip")
	require.False(t, locked, "only one instance may run a scheduler pass")
	require.Zero(t, processed, "a skipped pass archives nothing")
	require.Zero(t, readyArchives(t, ctx, pool, api.ClientID), "no archive is created while another instance holds the pass")
	release()

	runAt := time.Now()
	processed, locked, err = service.RunDue(ctx)
	require.NoError(t, err, "the scheduler pass archives due clients")
	require.True(t, locked, "the pass runs once the scheduler lock is free")
	require.Equal(t, 3, processed, "every due client is processed, including clients without eligible logs")
	require.Equal(t, 1, readyArchives(t, ctx, pool, api.ClientID), "the first client gets one archive")
	require.Equal(t, 1, readyArchives(t, ctx, pool, worker.ClientID), "the second client gets one archive")
	require.Zero(t, readyArchives(t, ctx, pool, idle.ClientID), "an empty automatic run creates no archive")
	for i, scope := range scopes {
		interval := time.Duration(i+1) * 7 * 24 * time.Hour
		require.WithinDuration(t, runAt.Add(interval), nextArchiveRun(t, ctx, pool, scope.ClientID), 5*time.Second, "each successful or empty run waits its own policy interval")
	}

	processed, _, err = service.RunDue(ctx)
	require.NoError(t, err, "a pass with nothing due succeeds")
	require.Zero(t, processed, "archived clients are not due again before their interval")

	_, err = pool.Exec(ctx, "UPDATE archive_policies SET next_run_at = now() - interval '1 second' WHERE client_id = $1", api.ClientID)
	require.NoError(t, err, "advance the first client's policy to its next automatic run")
	insertArchiveLog(t, ctx, pool, api, "old-while-busy")
	release = holdAdvisoryLock(t, ctx, pool, "svarog:archive:"+api.ClientID.String())
	runAt = time.Now()
	processed, _, err = service.RunDue(ctx)
	require.NoError(t, err, "a busy client does not fail the pass")
	require.Equal(t, 1, processed, "the due busy client is attempted")
	require.Equal(t, 1, readyArchives(t, ctx, pool, api.ClientID), "a busy client is not archived twice")
	require.WithinDuration(t, runAt.Add(15*time.Minute), nextArchiveRun(t, ctx, pool, api.ClientID), 5*time.Second, "a busy client retries in 15 minutes rather than waiting its full policy interval")
	release()

	_, err = pool.Exec(ctx, "UPDATE archive_policies SET next_run_at = now() - interval '1 second' WHERE client_id = $1", api.ClientID)
	require.NoError(t, err, "make the retained log eligible for another automatic attempt")
	release = gateArchiveQuery(t, ctx, pool, "archives", "status", "NEW.status = 'ready'", 71006)
	done := make(chan error, 1)
	go func() {
		_, _, err := service.RunDue(ctx)
		done <- err
	}()
	waitForArchiveQuery(t, ctx, pool, "MarkArchiveReady")
	require.NoError(t, service.SetPolicy(ctx, api, archive.Policy{Enabled: true, ArchiveAfterWeeks: 4}), "a policy can be changed while its old automatic build finishes")
	changedNext := nextArchiveRun(t, ctx, pool, api.ClientID)
	release()
	select {
	case err := <-done:
		require.NoError(t, err, "the running automatic archive completes after the policy change")
	case <-ctx.Done():
		t.Fatal("the automatic archive must finish before the test deadline")
	}
	require.Equal(t, 2, readyArchives(t, ctx, pool, api.ClientID), "the already claimed log finishes archiving under the original policy")
	require.Equal(t, changedNext, nextArchiveRun(t, ctx, pool, api.ClientID), "finishing an old automatic run preserves the deadline set by a concurrent policy change")
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

func nextArchiveRun(t *testing.T, ctx context.Context, pool *pgxpool.Pool, clientID uuid.UUID) time.Time {
	t.Helper()
	var next time.Time
	require.NoError(t, pool.QueryRow(ctx, "SELECT next_run_at FROM archive_policies WHERE client_id = $1", clientID).Scan(&next), "read the client's persisted automatic archive schedule")
	return next
}
