package postgres_test

import (
	"context"
	"strconv"
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

func TestReadLogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool)
	project, err := projectService.CreateProject(ctx, "LogsRead", []string{"api", "worker"})
	require.NoError(t, err, "create project and clients")
	apiID := clientByName(t, project, "api")
	workerID := clientByName(t, project, "worker")
	logs := db.NewLogService(nil, pool, nil)
	base := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	_, err = store.InsertLogs(ctx, pool, []store.LogInsert{
		{ClientID: apiID, InstanceName: "web", OccurredAt: base, SequenceNumber: 1, Message: "older"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: base.Add(10 * time.Second), SequenceNumber: 5, Message: "tie-low"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: base.Add(10 * time.Second), SequenceNumber: 5, Message: "tie-high"},
		{ClientID: apiID, InstanceName: "api", OccurredAt: base.Add(20 * time.Second), SequenceNumber: 8, Message: "instance-api"},
		{ClientID: apiID, InstanceName: "web", OccurredAt: base.Add(30 * time.Second), SequenceNumber: 9, Message: "newest"},
		{ClientID: workerID, InstanceName: "other-host", OccurredAt: base, SequenceNumber: 1, Message: "worker"},
	})
	require.NoError(t, err, "insert logs")

	api := project.ID.String()
	client := apiID.String()

	t.Run("first page is newest", func(t *testing.T) {
		page := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 2})
		require.Equal(t, []string{"newest", "instance-api"}, messages(page.Logs))
		require.True(t, page.IsLastPage)
		require.Nil(t, page.ForwardCursor)
		require.NotNil(t, page.BackwardCursor)
		require.Equal(t, page.Logs[1].ID, page.BackwardCursor.ID)
		require.Contains(t, page.ToPath(api, client, nil, page.BackwardCursor, "backward"), "cursorId="+strconv.FormatInt(page.Logs[1].ID, 10))
	})

	t.Run("backward page does not overlap", func(t *testing.T) {
		first := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 2})
		second := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 2, Cursor: first.BackwardCursor})
		require.Equal(t, []string{"tie-high", "tie-low"}, messages(second.Logs))
		require.False(t, second.IsLastPage)
		require.Nil(t, second.ForwardCursor)
		require.NotContains(t, messages(second.Logs), "newest")
		third := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 2, Cursor: second.BackwardCursor})
		require.Equal(t, []string{"older"}, messages(third.Logs))
	})

	t.Run("same timestamp and sequence split by id", func(t *testing.T) {
		all := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 10})
		tieHigh := findMessage(t, all.Logs, "tie-high")
		withID := mustPage(t, ctx, logs, db.LogPageRequest{
			ProjectId: api,
			ClientId:  client,
			PageSize:  1,
			Cursor: &db.LastCursor{
				Timestamp:      tieHigh.Timestamp,
				SequenceNumber: tieHigh.SequenceNumber,
				ID:             tieHigh.ID,
				IsBackward:     true,
			},
		})
		require.Equal(t, []string{"tie-low"}, messages(withID.Logs))

		withoutID := mustPage(t, ctx, logs, db.LogPageRequest{
			ProjectId: api,
			ClientId:  client,
			PageSize:  1,
			Cursor: &db.LastCursor{
				Timestamp:      tieHigh.Timestamp,
				SequenceNumber: tieHigh.SequenceNumber,
				IsBackward:     true,
			},
		})
		require.Equal(t, []string{"older"}, messages(withoutID.Logs))
	})

	t.Run("forward returns the newest newer rows", func(t *testing.T) {
		all := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 10})
		older := findMessage(t, all.Logs, "older")
		page := mustPage(t, ctx, logs, db.LogPageRequest{
			ProjectId: api,
			ClientId:  client,
			PageSize:  2,
			Cursor: &db.LastCursor{
				Timestamp:      older.Timestamp,
				SequenceNumber: older.SequenceNumber,
				ID:             older.ID,
				IsBackward:     false,
			},
		})
		require.Equal(t, []string{"newest", "instance-api"}, messages(page.Logs))
		require.NotNil(t, page.ForwardCursor)
		require.False(t, page.IsLastPage)
	})

	t.Run("wrong project and invalid id are empty", func(t *testing.T) {
		wrong := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: uuid.New().String(), ClientId: client, PageSize: 10})
		require.Empty(t, wrong.Logs)
		require.True(t, wrong.IsLastPage)
		invalid := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: "marko", PageSize: 10})
		require.Empty(t, invalid.Logs)
		require.True(t, invalid.IsLastPage)
		cursor := &db.LastCursor{Timestamp: base.Add(time.Hour), SequenceNumber: 1, ID: 1, IsBackward: true}
		invalidCursor := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: "marko", PageSize: 10, Cursor: cursor})
		require.Empty(t, invalidCursor.Logs)
		require.False(t, invalidCursor.IsLastPage)
	})

	t.Run("instance filter", func(t *testing.T) {
		all := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 10})
		require.Len(t, all.Logs, 5)
		web := []string{"web"}
		filtered := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, Instances: &web, PageSize: 10})
		require.Equal(t, []string{"newest", "tie-high", "tie-low", "older"}, messages(filtered.Logs))
		empty := []string{}
		none := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, Instances: &empty, PageSize: 10})
		require.Empty(t, none.Logs)

		instances, err := logs.GetInstances(ctx, api, client)
		require.NoError(t, err)
		require.Equal(t, []string{"api", "web"}, instances)
		workerInstances, err := logs.GetInstances(ctx, api, workerID.String())
		require.NoError(t, err)
		require.Equal(t, []string{"other-host"}, workerInstances)
		missing, err := logs.GetInstances(ctx, api, "marko")
		require.NoError(t, err)
		require.Empty(t, missing)
	})

	t.Run("log line is the newest row on the page", func(t *testing.T) {
		all := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 10})
		target := findMessage(t, all.Logs, "instance-api")
		logLine := strconv.FormatInt(target.ID, 10)
		page := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 3, LogLineId: &logLine})
		require.Equal(t, []string{"instance-api", "tie-high", "tie-low"}, messages(page.Logs))
		require.True(t, page.IsLastPage)

		web := []string{"web"}
		filtered := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, Instances: &web, PageSize: 3, LogLineId: &logLine})
		require.Equal(t, []string{"tie-high", "tie-low", "older"}, messages(filtered.Logs))

		workerPage := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: workerID.String(), PageSize: 10})
		workerLine := strconv.FormatInt(findMessage(t, workerPage.Logs, "worker").ID, 10)
		foreign := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 2, LogLineId: &workerLine})
		require.Equal(t, []string{"newest", "instance-api"}, messages(foreign.Logs))
		bad := "not-an-id"
		invalid := mustPage(t, ctx, logs, db.LogPageRequest{ProjectId: api, ClientId: client, PageSize: 2, LogLineId: &bad})
		require.Equal(t, []string{"newest", "instance-api"}, messages(invalid.Logs))
	})

	t.Run("inactive client stays readable", func(t *testing.T) {
		inactiveProject, err := projectService.CreateProject(ctx, "LogsInactive", []string{"api"})
		require.NoError(t, err)
		inactiveClient := clientByName(t, inactiveProject, "api")
		_, err = store.InsertLogs(ctx, pool, []store.LogInsert{{
			ClientID: inactiveClient, InstanceName: "web", OccurredAt: base, SequenceNumber: 1, Message: "kept",
		}})
		require.NoError(t, err)
		_, err = projectService.UpdateProject(ctx, inactiveProject.ID, inactiveProject.Name, nil)
		require.NoError(t, err, "deactivate client while retaining its row")
		page := mustPage(t, ctx, logs, db.LogPageRequest{
			ProjectId: inactiveProject.ID.String(),
			ClientId:  inactiveClient.String(),
			PageSize:  10,
		})
		require.Equal(t, []string{"kept"}, messages(page.Logs))
		instances, err := logs.GetInstances(ctx, inactiveProject.ID.String(), inactiveClient.String())
		require.NoError(t, err)
		require.Equal(t, []string{"web"}, instances)
	})
}

func clientByName(t *testing.T, project projects.Project, name string) uuid.UUID {
	t.Helper()
	for _, client := range project.Clients {
		if client.Name == name {
			return client.ID
		}
	}
	t.Fatalf("client %s not found", name)
	return uuid.Nil
}

func mustPage(t *testing.T, ctx context.Context, logs *db.MongoLogService, req db.LogPageRequest) db.LogPage {
	t.Helper()
	page, err := logs.GetLogs(ctx, req)
	require.NoError(t, err)
	return page
}

func messages(logs []types.StoredLog) []string {
	out := make([]string, len(logs))
	for i, log := range logs {
		out[i] = log.LogLine
	}
	return out
}

func findMessage(t *testing.T, logs []types.StoredLog, message string) types.StoredLog {
	t.Helper()
	for _, log := range logs {
		if log.LogLine == message {
			return log
		}
	}
	t.Fatalf("log %s not found", message)
	return types.StoredLog{}
}
