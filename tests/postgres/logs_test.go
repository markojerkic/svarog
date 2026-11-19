package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestCommitLogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool)
	project, err := projectService.Create(ctx, projects.ProjectInput{Name: "Idempotent", Clients: []string{"api", "retired"}})
	require.NoError(t, err, "create a client for idempotent log writes")
	clientID := clientByName(t, project, "api")
	logs := logstore.New(pool)
	subject := "logs." + project.ID.String() + "." + clientID.String()
	line := func(message string) logstore.Event {
		return logstore.Event{
			Scope:    logstore.Scope{ProjectID: project.ID, ClientID: clientID},
			Key:      logevent.MessageKey(subject, []byte(message)),
			Instance: "web", OccurredAt: time.Now().UTC(), Message: message,
		}
	}

	t.Run("empty batch and persisted metadata", func(t *testing.T) {
		results, err := logs.Commit(ctx, nil)
		require.NoError(t, err, "an empty commit must succeed")
		require.Empty(t, results, "an empty commit must return no outcomes")
		var count int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM logs").Scan(&count), "count logs after the empty commit")
		require.Zero(t, count, "an empty commit must not insert a row")

		event := line("new row metadata")
		results, err = logs.Commit(ctx, []logstore.Event{event})
		require.NoError(t, err, "commit a row for the active client")
		require.Len(t, results, 1, "the event must have one outcome")
		require.Equal(t, logstore.Inserted, results[0].Outcome, "the event must be inserted")
		var archiveID *uuid.UUID
		var receivedAt time.Time
		var message string
		require.NoError(t, pool.QueryRow(ctx, "SELECT archive_id, received_at, message FROM logs WHERE id = $1", results[0].Row.ID).Scan(&archiveID, &receivedAt, &message), "read metadata of the inserted log by its ID")
		require.Nil(t, archiveID, "a new log has not been archived")
		require.False(t, receivedAt.IsZero(), "PostgreSQL must set the receipt time")
		require.Equal(t, event.Message, message, "the returned ID must identify the inserted event")
	})

	t.Run("deduplicates batch and later deliveries without losing input positions", func(t *testing.T) {
		a, b := line("first"), line("second")
		results, err := logs.Commit(ctx, []logstore.Event{a, b, a})
		require.NoError(t, err, "commit a batch containing a repeated event")
		require.Len(t, results, 3, "every input receives an outcome")
		require.Positive(t, results[0].Row.ID, "the first new row is assigned a database ID")
		require.Positive(t, results[1].Row.ID, "the second new row is assigned a database ID")
		require.Equal(t, []logstore.Result{
			{Outcome: logstore.Inserted, Row: insertedRow(a, results[0].Row.ID)},
			{Outcome: logstore.Inserted, Row: insertedRow(b, results[1].Row.ID)},
			{Outcome: logstore.Duplicate},
		}, results, "the first occurrence of each event is inserted and the repeated input is a duplicate")
		repeated, err := logs.Commit(ctx, []logstore.Event{a, b})
		require.NoError(t, err, "a later delivery of committed events is successful")
		require.Equal(t, []logstore.Result{
			{Outcome: logstore.Duplicate},
			{Outcome: logstore.Duplicate},
		}, repeated, "a later delivery reports duplicates without new row IDs")
		_, err = pool.Exec(ctx, "DELETE FROM logs WHERE id=$1", results[0].Row.ID)
		require.NoError(t, err, "remove the log as archiving does")
		repeated, err = logs.Commit(ctx, []logstore.Event{a})
		require.NoError(t, err, "event keys intentionally do not survive deletion")
		require.Len(t, repeated, 1, "the retried event has an outcome")
		require.Equal(t, logstore.Inserted, repeated[0].Outcome, "a deleted event may be stored again without a permanent receipt")
	})

	t.Run("concurrent replicas share the unique key", func(t *testing.T) {
		event := line("concurrent")
		start := make(chan struct{})
		var wg sync.WaitGroup
		results := make([][]logstore.Result, 2)
		errors := make([]error, 2)
		for i := range 2 {
			wg.Go(func() {
				<-start
				results[i], errors[i] = logs.Commit(ctx, []logstore.Event{event})
			})
		}
		close(start)
		wg.Wait()
		for i, err := range errors {
			require.NoError(t, err, "both concurrent deliveries commit successfully")
			require.Len(t, results[i], 1, "both replicas report one outcome for their delivery")
		}
		require.ElementsMatch(t, []logstore.Outcome{logstore.Inserted, logstore.Duplicate},
			[]logstore.Outcome{results[0][0].Outcome, results[1][0].Outcome}, "exactly one replica inserts the new log")
	})

	t.Run("failed batch rolls back all event keys", func(t *testing.T) {
		good, bad := line("rollback good"), line("rollback invalid sequence")
		bad.Sequence = -1
		_, err := logs.Commit(ctx, []logstore.Event{good, bad})
		require.ErrorIs(t, err, logstore.ErrRowData, "an invalid sequence must classify the failed statement as row data")
		var pgError *pgconn.PgError
		require.ErrorAs(t, err, &pgError, "the commit error must preserve the original PostgreSQL error")
		require.Equal(t, "23514", pgError.Code, "the statement must fail on a check constraint")
		require.Equal(t, "logs_sequence_number_check", pgError.ConstraintName, "the invalid sequence must be the source of the failed commit")
		results, err := logs.Commit(ctx, []logstore.Event{good})
		require.NoError(t, err, "a rolled-back event may be retried")
		require.Len(t, results, 1, "the retried event has an outcome")
		require.Equal(t, logstore.Inserted, results[0].Outcome, "failed atomic writes must not reserve keys or lose valid rows")
	})

	t.Run("reports mixed outcomes at the original input ordinals", func(t *testing.T) {
		retiredID := clientByName(t, project, "retired")
		_, err := projectService.Update(ctx, project.ID, projects.ProjectInput{Name: project.Name, Clients: []string{"api"}})
		require.NoError(t, err, "deactivate the retired client while keeping the API client active")
		other, err := projectService.Create(ctx, projects.ProjectInput{Name: "Other ingest project", Clients: []string{"foreign"}})
		require.NoError(t, err, "create an active client belonging to another project")

		stored, fresh := line("mixed stored"), line("mixed new")
		_, err = logs.Commit(ctx, []logstore.Event{stored})
		require.NoError(t, err, "seed the stored event before the mixed batch")
		var before int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM logs").Scan(&before), "count retained logs before the mixed batch")

		foreign, inactive, unknown := line("mixed foreign"), line("mixed inactive"), line("mixed unknown")
		foreign.Scope.ClientID = other.Clients[0].ID
		inactive.Scope.ClientID = retiredID
		unknown.Scope.ClientID = uuid.New()
		results, err := logs.Commit(ctx, []logstore.Event{foreign, fresh, stored, inactive, fresh, unknown})
		require.NoError(t, err, "commit new events while classifying duplicates and invalid clients")
		require.Len(t, results, 6, "the mixed batch returns an outcome for every delivery")
		require.Positive(t, results[1].Row.ID, "the freshly inserted event has a database ID")
		require.Equal(t, []logstore.Result{
			{Outcome: logstore.RejectedClient},
			{Outcome: logstore.Inserted, Row: insertedRow(fresh, results[1].Row.ID)},
			{Outcome: logstore.Duplicate},
			{Outcome: logstore.RejectedClient},
			{Outcome: logstore.Duplicate},
			{Outcome: logstore.RejectedClient},
		}, results, "each outcome maps to its original ordinal and only the new event receives an ID")

		var after int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM logs").Scan(&after), "count retained logs after the mixed batch")
		require.Equal(t, before+1, after, "the mixed batch stores exactly one additional row")
		var message string
		require.NoError(t, pool.QueryRow(ctx, "SELECT message FROM logs WHERE id = $1", results[1].Row.ID).Scan(&message), "read the row by the assigned ID")
		require.Equal(t, fresh.Message, message, "the assigned ID identifies the new active-client event")
	})
}

func insertedRow(event logstore.Event, id int64) logstore.Row {
	return logstore.Row{ID: id, Scope: event.Scope, Instance: event.Instance, OccurredAt: event.OccurredAt, Sequence: event.Sequence, Message: event.Message}
}
