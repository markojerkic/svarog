package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/projects"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestInsertLogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool)
	project, err := projectService.CreateProject(ctx, "Logs", []string{"api"})
	require.NoError(t, err, "create project and client for log inserts")
	clientID := project.Clients[0].ID
	occurredAt := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	ids, err := store.InsertLogs(ctx, pool, nil)

	require.NoError(t, err, "empty log batch must not fail")
	require.Empty(t, ids, "empty log batch must not return ids")
	var count int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM logs").Scan(&count), "count logs after empty insert")
	require.Zero(t, count, "empty log batch must not insert a row")

	ids, err = store.InsertLogs(ctx, pool, []store.LogInsert{
		{ClientID: clientID, InstanceName: "web", OccurredAt: occurredAt, SequenceNumber: 1, Message: "first"},
		{ClientID: clientID, InstanceName: "api", OccurredAt: occurredAt.Add(time.Second), SequenceNumber: 2, Message: "second"},
	})

	require.NoError(t, err, "insert two logs for an active client")
	require.Len(t, ids, 2, "insert returns one id per input row")
	require.Less(t, ids[0], ids[1], "returned ids follow input order")
	var archiveID *uuid.UUID
	var receivedAt time.Time
	var message string
	require.NoError(t, pool.QueryRow(ctx, "SELECT archive_id, received_at, message FROM logs WHERE id = $1", ids[0]).Scan(&archiveID, &receivedAt, &message), "read inserted log")
	require.Nil(t, archiveID, "new logs are not archived")
	require.False(t, receivedAt.IsZero(), "received_at is set by the database")
	require.Equal(t, "first", message, "first returned id belongs to the first input row")

	_, err = projectService.UpdateProject(ctx, project.ID, project.Name, nil)
	require.NoError(t, err, "deactivate client while retaining its row")
	inactiveIDs, err := store.InsertLogs(ctx, pool, []store.LogInsert{{
		ClientID: clientID, InstanceName: "web", OccurredAt: occurredAt, SequenceNumber: 3, Message: "inactive still stored",
	}})
	require.NoError(t, err, "inactive client row still satisfies the log foreign key")
	require.Len(t, inactiveIDs, 1, "inactive client log is inserted")

	_, err = store.InsertLogs(ctx, pool, []store.LogInsert{{
		ClientID: uuid.New(), InstanceName: "web", OccurredAt: occurredAt, SequenceNumber: 4, Message: "missing",
	}})
	require.True(t, store.IsConstraintViolation(err, store.SQLStateForeignKeyViolation, "logs_client_id_fkey"), "missing client must fail the logs foreign key")
}

func TestInsertIngestLogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	project, err := projects.NewPostgresProjectsService(pool).CreateProject(ctx, "Idempotent", []string{"api"})
	require.NoError(t, err, "create a client for idempotent log writes")
	clientID := project.Clients[0].ID
	subject := "logs." + project.ID.String() + "." + clientID.String()
	line := func(message string) store.IngestLogInsert {
		return store.IngestLogInsert{
			EventKey: rpc.MessageKey(subject, []byte(message)),
			Log:      store.LogInsert{ClientID: clientID, InstanceName: "web", OccurredAt: time.Now().UTC(), Message: message},
		}
	}

	t.Run("deduplicates batch and later deliveries without losing input positions", func(t *testing.T) {
		a, b := line("first"), line("second")
		inserted, err := store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{a, b, a})
		require.NoError(t, err, "commit a batch containing a repeated event")
		require.Len(t, inserted, 2, "a repeated event produces only one stored row")
		require.Equal(t, int64(1), inserted[0].Ordinal, "the first new row maps to the first input")
		require.Equal(t, int64(2), inserted[1].Ordinal, "the second new row maps to the second input")
		repeated, err := store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{a, b})
		require.NoError(t, err, "a later delivery of committed events is successful")
		require.Empty(t, repeated, "a later delivery emits no new rows or live candidates")
		_, err = pool.Exec(ctx, "DELETE FROM logs WHERE id=$1", inserted[0].ID)
		require.NoError(t, err, "remove the log as archiving does")
		repeated, err = store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{a})
		require.NoError(t, err, "event keys intentionally do not survive deletion")
		require.Len(t, repeated, 1, "a deleted event may be stored again without a permanent receipt")
	})

	t.Run("concurrent replicas share the unique key", func(t *testing.T) {
		event := line("concurrent")
		start := make(chan struct{})
		var wg sync.WaitGroup
		results := make([][]store.InsertedIngestLog, 2)
		errors := make([]error, 2)
		for i := range 2 {
			wg.Go(func() {
				<-start
				results[i], errors[i] = store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{event})
			})
		}
		close(start)
		wg.Wait()
		for _, err := range errors {
			require.NoError(t, err, "both concurrent deliveries commit successfully")
		}
		require.Equal(t, 1, len(results[0])+len(results[1]), "only one replica inserts and publishes the new log")
	})

	t.Run("failed batch rolls back all event keys", func(t *testing.T) {
		good, bad := line("rollback good"), line("rollback orphan")
		bad.Log.ClientID = uuid.New()
		_, err := store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{good, bad})
		require.True(t, store.IsConstraintViolation(err, store.SQLStateForeignKeyViolation, "logs_client_id_fkey"), "an orphan makes the entire statement fail")
		inserted, err := store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{good})
		require.NoError(t, err, "a rolled-back event may be retried")
		require.Len(t, inserted, 1, "failed atomic writes must not reserve keys or lose valid rows")
	})
}
