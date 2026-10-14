package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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
