package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/projects"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestSearchLogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool)
	project, err := projectService.CreateProject(ctx, "LogsSearch", []string{"api", "worker"})
	require.NoError(t, err, "create project and clients for log search")
	apiID := clientByName(t, project, "api")
	workerID := clientByName(t, project, "worker")
	logs := db.NewLogService(pool, nil)
	base := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	_, err = store.InsertLogs(ctx, pool, []store.LogInsert{
		{ClientID: apiID, InstanceName: "web", OccurredAt: base, SequenceNumber: 1, Message: "older connection refused"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: base.Add(5 * time.Second), SequenceNumber: 2, Message: "foo only"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: base.Add(10 * time.Second), SequenceNumber: 5, Message: "needle low"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: base.Add(10 * time.Second), SequenceNumber: 5, Message: "needle high"},
		{ClientID: apiID, InstanceName: "api", OccurredAt: base.Add(20 * time.Second), SequenceNumber: 8, Message: "connection timeout"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: base.Add(30 * time.Second), SequenceNumber: 9, Message: "Error code 500"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: base.Add(40 * time.Second), SequenceNumber: 10, Message: "foo-bar happened"},
		{ClientID: workerID, InstanceName: "other-host", OccurredAt: base, SequenceNumber: 1, Message: "connection refused elsewhere"},
	})
	require.NoError(t, err, "insert searchable logs")

	api := project.ID.String()
	client := apiID.String()

	t.Run("both words are required and other clients stay hidden", func(t *testing.T) {
		found := mustSearch(t, ctx, logs, "connection refused", api, client, nil, 10, nil)
		require.Equal(t, []string{"older connection refused"}, messages(found), "search requires every word and ignores another client's match")
		oneWord := mustSearch(t, ctx, logs, "connection", api, client, nil, 10, nil)
		require.Equal(t, []string{"connection timeout", "older connection refused"}, messages(oneWord), "a single word matches every client log containing it, newest first")
		require.Equal(t, []string{"Error code 500"}, messages(mustSearch(t, ctx, logs, "error", api, client, nil, 10, nil)), "simple text search is case insensitive")
		require.Empty(t, mustSearch(t, ctx, logs, "err", api, client, nil, 10, nil), "a prefix is not a match")
	})

	t.Run("hyphen is not an exclusion operator", func(t *testing.T) {
		require.Equal(t, []string{"foo-bar happened"}, messages(mustSearch(t, ctx, logs, "foo-bar", api, client, nil, 10, nil)), "a hyphenated query matches the log that contains both words")
		require.Equal(t, []string{"foo-bar happened", "foo only"}, messages(mustSearch(t, ctx, logs, "foo", api, client, nil, 10, nil)), "either side of a hyphen remains searchable")
		require.Equal(t, []string{"foo-bar happened"}, messages(mustSearch(t, ctx, logs, "bar", api, client, nil, 10, nil)), "the word after a hyphen is not treated as excluded")
	})

	t.Run("empty query and wrong scope return nothing", func(t *testing.T) {
		require.Empty(t, mustSearch(t, ctx, logs, "", api, client, nil, 10, nil), "an empty search matches no logs")
		require.Empty(t, mustSearch(t, ctx, logs, "   ", api, client, nil, 10, nil), "a blank search matches no logs")
		require.Empty(t, mustSearch(t, ctx, logs, "connection", uuid.New().String(), client, nil, 10, nil), "a client from another project is not searchable")
		require.Empty(t, mustSearch(t, ctx, logs, "connection", api, "marko", nil, 10, nil), "an invalid client id returns no logs")
	})

	t.Run("instance filter and cursor id", func(t *testing.T) {
		apiInstance := []string{"api"}
		require.Equal(t, []string{"connection timeout"}, messages(mustSearch(t, ctx, logs, "connection", api, client, &apiInstance, 10, nil)), "instance filter keeps only the named instance")
		empty := []string{}
		require.Empty(t, mustSearch(t, ctx, logs, "connection", api, client, &empty, 10, nil), "an empty instance list matches nothing")

		needles := mustSearch(t, ctx, logs, "needle", api, client, nil, 1, nil)
		require.Equal(t, []string{"needle high"}, messages(needles), "same timestamp and sequence sorts the higher id first")
		withID := mustSearch(t, ctx, logs, "needle", api, client, nil, 1, &db.LastCursor{
			Timestamp:      needles[0].Timestamp,
			SequenceNumber: needles[0].SequenceNumber,
			ID:             needles[0].ID,
			IsBackward:     true,
		})
		require.Equal(t, []string{"needle low"}, messages(withID), "cursor id returns the other row with the same timestamp and sequence")
		withoutID := mustSearch(t, ctx, logs, "needle", api, client, nil, 1, &db.LastCursor{
			Timestamp:      needles[0].Timestamp,
			SequenceNumber: needles[0].SequenceNumber,
			IsBackward:     true,
		})
		require.Empty(t, withoutID, "a cursor without id skips the tied row")
	})

	t.Run("inactive client stays searchable", func(t *testing.T) {
		inactiveProject, err := projectService.CreateProject(ctx, "LogsSearchInactive", []string{"api"})
		require.NoError(t, err, "create project for inactive search")
		inactiveClient := clientByName(t, inactiveProject, "api")
		_, err = store.InsertLogs(ctx, pool, []store.LogInsert{{
			ClientID: inactiveClient, InstanceName: "web", OccurredAt: base, SequenceNumber: 1, Message: "kept secret",
		}})
		require.NoError(t, err, "insert a log before deactivating the client")
		_, err = projectService.UpdateProject(ctx, inactiveProject.ID, inactiveProject.Name, nil)
		require.NoError(t, err, "deactivate client while retaining its row")
		found := mustSearch(t, ctx, logs, "secret", inactiveProject.ID.String(), inactiveClient.String(), nil, 10, nil)
		require.Equal(t, []string{"kept secret"}, messages(found), "deactivating a client does not hide its logs from search")
	})
}

func mustSearch(t *testing.T, ctx context.Context, logs *db.PostgresLogService, search, projectID, clientID string, instances *[]string, pageSize int64, cursor *db.LastCursor) []types.StoredLog {
	t.Helper()
	found, err := logs.SearchLogs(ctx, search, projectID, clientID, instances, pageSize, cursor)
	require.NoError(t, err, "search %q", search)
	return found
}
