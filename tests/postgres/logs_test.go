package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/lib/projects"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
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
	projectService := projects.NewPostgresProjectsService(pool)
	project, err := projectService.CreateProject(ctx, "Idempotent", []string{"api", "retired"})
	require.NoError(t, err, "create a client for idempotent log writes")
	clientID := clientByName(t, project, "api")
	subject := "logs." + project.ID.String() + "." + clientID.String()
	line := func(message string) store.IngestLogInsert {
		return store.IngestLogInsert{
			ProjectID: project.ID,
			EventKey:  logevent.MessageKey(subject, []byte(message)),
			Log:       store.LogInsert{ClientID: clientID, InstanceName: "web", OccurredAt: time.Now().UTC(), Message: message},
		}
	}

	t.Run("deduplicates batch and later deliveries without losing input positions", func(t *testing.T) {
		a, b := line("first"), line("second")
		results, err := store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{a, b, a})
		require.NoError(t, err, "commit a batch containing a repeated event")
		require.Len(t, results, 3, "every input receives an outcome")
		require.Positive(t, results[0].ID, "the first new row is assigned a database ID")
		require.Positive(t, results[1].ID, "the second new row is assigned a database ID")
		require.Equal(t, []store.IngestLogResult{
			{Ordinal: 1, ID: results[0].ID, Status: store.IngestLogInserted},
			{Ordinal: 2, ID: results[1].ID, Status: store.IngestLogInserted},
			{Ordinal: 3, Status: store.IngestLogDuplicate},
		}, results, "the first occurrence of each event is inserted and the repeated input is a duplicate")
		repeated, err := store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{a, b})
		require.NoError(t, err, "a later delivery of committed events is successful")
		require.Equal(t, []store.IngestLogResult{
			{Ordinal: 1, Status: store.IngestLogDuplicate},
			{Ordinal: 2, Status: store.IngestLogDuplicate},
		}, repeated, "a later delivery reports duplicates without new row IDs")
		_, err = pool.Exec(ctx, "DELETE FROM logs WHERE id=$1", results[0].ID)
		require.NoError(t, err, "remove the log as archiving does")
		repeated, err = store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{a})
		require.NoError(t, err, "event keys intentionally do not survive deletion")
		require.Len(t, repeated, 1, "the retried event has an outcome")
		require.Equal(t, store.IngestLogInserted, repeated[0].Status, "a deleted event may be stored again without a permanent receipt")
	})

	t.Run("concurrent replicas share the unique key", func(t *testing.T) {
		event := line("concurrent")
		start := make(chan struct{})
		var wg sync.WaitGroup
		results := make([][]store.IngestLogResult, 2)
		errors := make([]error, 2)
		for i := range 2 {
			wg.Go(func() {
				<-start
				results[i], errors[i] = store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{event})
			})
		}
		close(start)
		wg.Wait()
		for i, err := range errors {
			require.NoError(t, err, "both concurrent deliveries commit successfully")
			require.Len(t, results[i], 1, "both replicas report one outcome for their delivery")
		}
		require.ElementsMatch(t, []store.IngestLogStatus{store.IngestLogInserted, store.IngestLogDuplicate},
			[]store.IngestLogStatus{results[0][0].Status, results[1][0].Status}, "exactly one replica inserts the new log")
	})

	t.Run("failed batch rolls back all event keys", func(t *testing.T) {
		good, bad := line("rollback good"), line("rollback invalid sequence")
		bad.Log.SequenceNumber = -1
		_, err := store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{good, bad})
		require.True(t, store.IsConstraintViolation(err, "23514", "logs_sequence_number_check"), "an invalid sequence makes the entire statement fail")
		results, err := store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{good})
		require.NoError(t, err, "a rolled-back event may be retried")
		require.Len(t, results, 1, "the retried event has an outcome")
		require.Equal(t, store.IngestLogInserted, results[0].Status, "failed atomic writes must not reserve keys or lose valid rows")
	})

	t.Run("reports mixed outcomes at the original input ordinals", func(t *testing.T) {
		retiredID := clientByName(t, project, "retired")
		_, err := projectService.UpdateProject(ctx, project.ID, project.Name, []string{"api"})
		require.NoError(t, err, "deactivate the retired client while keeping the API client active")
		other, err := projectService.CreateProject(ctx, "Other ingest project", []string{"foreign"})
		require.NoError(t, err, "create an active client belonging to another project")

		stored, fresh := line("mixed stored"), line("mixed new")
		_, err = store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{stored})
		require.NoError(t, err, "seed the stored event before the mixed batch")
		var before int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM logs").Scan(&before), "count retained logs before the mixed batch")

		foreign, inactive, unknown := line("mixed foreign"), line("mixed inactive"), line("mixed unknown")
		foreign.Log.ClientID = other.Clients[0].ID
		inactive.Log.ClientID = retiredID
		unknown.Log.ClientID = uuid.New()
		results, err := store.InsertIngestLogs(ctx, pool, []store.IngestLogInsert{foreign, fresh, stored, inactive, fresh, unknown})
		require.NoError(t, err, "commit new events while classifying duplicates and invalid clients")
		require.Len(t, results, 6, "the mixed batch returns an outcome for every delivery")
		require.Positive(t, results[1].ID, "the freshly inserted event has a database ID")
		require.Equal(t, []store.IngestLogResult{
			{Ordinal: 1, Status: store.IngestLogRejectedClient},
			{Ordinal: 2, ID: results[1].ID, Status: store.IngestLogInserted},
			{Ordinal: 3, Status: store.IngestLogDuplicate},
			{Ordinal: 4, Status: store.IngestLogRejectedClient},
			{Ordinal: 5, Status: store.IngestLogDuplicate},
			{Ordinal: 6, Status: store.IngestLogRejectedClient},
		}, results, "each outcome maps to its original ordinal and only the new event receives an ID")

		var after int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM logs").Scan(&after), "count retained logs after the mixed batch")
		require.Equal(t, before+1, after, "the mixed batch stores exactly one additional row")
		var message string
		require.NoError(t, pool.QueryRow(ctx, "SELECT message FROM logs WHERE id = $1", results[1].ID).Scan(&message), "read the row by the assigned ID")
		require.Equal(t, fresh.Log.Message, message, "the assigned ID identifies the new active-client event")
	})
}
