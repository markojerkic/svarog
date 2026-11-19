package postgres_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
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
	logs := logstore.New(pool)
	base := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	_, err = testutils.InsertLogFixtures(ctx, pool, []logstore.Row{
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base, Sequence: 1, Message: "older"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(10 * time.Second), Sequence: 5, Message: "tie-low"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(10 * time.Second), Sequence: 5, Message: "tie-high"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "api", OccurredAt: base.Add(20 * time.Second), Sequence: 8, Message: "instance-api"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(30 * time.Second), Sequence: 9, Message: "newest"},
		{Scope: logstore.Scope{ClientID: workerID}, Instance: "other-host", OccurredAt: base, Sequence: 1, Message: "worker"},
	})
	require.NoError(t, err, "insert logs")

	scope := logstore.Scope{ProjectID: project.ID, ClientID: apiID}

	t.Run("first page is newest", func(t *testing.T) {
		page := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Limit: 2})
		require.Equal(t, []string{"newest", "instance-api"}, messages(page.Rows), "the first page must show the newest rows")
		require.Nil(t, page.Newer, "the first page must not offer a newer cursor")
		require.NotNil(t, page.Older, "a full first page must offer older rows")
		require.Equal(t, page.Rows[1].ID, page.Older.ID, "the older cursor must use the oldest visible row")
		require.Contains(t, (pages.LogsPageProps{ProjectId: scope.ProjectID.String(), ClientId: scope.ClientID.String()}).ToPath(nil, page.Older, "backward", ""), "cursorId="+strconv.FormatInt(page.Rows[1].ID, 10), "pagination links must include the row id as a tie breaker")
	})

	t.Run("backward page does not overlap", func(t *testing.T) {
		first := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Limit: 2})
		second := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Limit: 2, Older: first.Older})
		require.Equal(t, []string{"tie-high", "tie-low"}, messages(second.Rows), "the second page must contain the adjacent older rows")
		require.Nil(t, second.Newer, "an older fragment must only continue backward")
		require.NotContains(t, messages(second.Rows), "newest", "the older page must not repeat a row from the first page")
		third := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Limit: 2, Older: second.Older})
		require.Equal(t, []string{"older"}, messages(third.Rows), "the last page must contain only the oldest row")
	})

	t.Run("same timestamp and sequence split by id", func(t *testing.T) {
		all := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Limit: 10})
		tieHigh := findMessage(t, all.Rows, "tie-high")
		withID := mustPage(t, ctx, logs, logstore.PageQuery{
			Scope: scope,
			Limit: 1,
			Older: &logstore.Cursor{
				OccurredAt: tieHigh.OccurredAt,
				Sequence:   tieHigh.Sequence,
				ID:         tieHigh.ID,
			},
		})
		require.Equal(t, []string{"tie-low"}, messages(withID.Rows), "the ID must keep the tied row reachable across pages")
	})

	t.Run("forward returns the nearest newer rows", func(t *testing.T) {
		all := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Limit: 10})
		older := findMessage(t, all.Rows, "older")
		page := mustPage(t, ctx, logs, logstore.PageQuery{
			Scope: scope,
			Limit: 2,
			Newer: &logstore.Cursor{
				OccurredAt: older.OccurredAt,
				Sequence:   older.Sequence,
				ID:         older.ID,
			},
		})
		require.Equal(t, []string{"tie-high", "tie-low"}, messages(page.Rows), "forward pagination must select adjacent newer rows and keep descending display order")
		require.NotNil(t, page.Newer, "a full forward page must offer the next newer rows")
	})

	t.Run("wrong project and missing scope are empty", func(t *testing.T) {
		wrong := mustPage(t, ctx, logs, logstore.PageQuery{Scope: logstore.Scope{ProjectID: uuid.New(), ClientID: apiID}, Limit: 10})
		require.Empty(t, wrong.Rows, "a foreign project must not see the client's logs")
		missing := logstore.Scope{ProjectID: project.ID}
		invalid := mustPage(t, ctx, logs, logstore.PageQuery{Scope: missing, Limit: 10})
		require.Empty(t, invalid.Rows, "a missing client must have no history")
		cursor := &logstore.Cursor{OccurredAt: base.Add(time.Hour), Sequence: 1, ID: 1}
		invalidCursor := mustPage(t, ctx, logs, logstore.PageQuery{Scope: missing, Limit: 10, Older: cursor})
		require.Empty(t, invalidCursor.Rows, "pagination must not return rows for a missing client")
	})

	t.Run("instance filter", func(t *testing.T) {
		all := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Limit: 10})
		require.Len(t, all.Rows, 5, "all five rows in the client must be readable")
		web := []string{"web"}
		filtered := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Instances: web, Limit: 10})
		require.Equal(t, []string{"newest", "tie-high", "tie-low", "older"}, messages(filtered.Rows), "filtering by instance must exclude unrelated rows")
		empty := []string{}
		none := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Instances: empty, Limit: 10})
		require.Empty(t, none.Rows, "a non-nil empty instance list must match nothing")

		instances, err := logs.Instances(ctx, scope)
		require.NoError(t, err, "the scoped log operation must succeed")
		require.Equal(t, []string{"api", "web"}, instances, "instance names must be sorted and distinct")
		workerInstances, err := logs.Instances(ctx, logstore.Scope{ProjectID: project.ID, ClientID: workerID})
		require.NoError(t, err, "the scoped log operation must succeed")
		require.Equal(t, []string{"other-host"}, workerInstances, "instance names must be limited to the requested client")
		missing, err := logs.Instances(ctx, logstore.Scope{ProjectID: project.ID})
		require.NoError(t, err, "the scoped log operation must succeed")
		require.Empty(t, missing, "a missing client must have no instance names")
		foreign, err := logs.Instances(ctx, logstore.Scope{ProjectID: uuid.New(), ClientID: apiID})
		require.NoError(t, err, "instance lookup with a mismatched project must complete safely")
		require.Empty(t, foreign, "instance names must not leak when the client belongs to another project")
	})

	t.Run("log context uses the authorized anchor instance", func(t *testing.T) {
		all := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Limit: 10})
		target := findMessage(t, all.Rows, "instance-api")
		page, found, err := logs.Around(ctx, scope, target.ID, 3, nil)
		require.NoError(t, err, "load the anchor and its instance context")
		require.True(t, found, "the anchor belongs to this client")
		require.Equal(t, []string{"instance-api"}, messages(page.Rows), "an instance with one log must show only its anchor")
		require.Nil(t, page.Newer, "an anchor with no newer logs cannot offer newer pages")

		tied := findMessage(t, all.Rows, "tie-low")
		tiedPage, found, err := logs.Around(ctx, scope, tied.ID, 1, nil)
		require.NoError(t, err, "load a tied anchor and its closest neighbors")
		require.True(t, found, "the tied anchor belongs to this client")
		require.Equal(t, []string{"tie-high", "tie-low", "older"}, messages(tiedPage.Rows), "context must include tied neighbors in unique row id order")
		newer := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Instances: []string{"api"}, AnchorID: tied.ID, Limit: 1, Newer: tiedPage.Newer})
		require.Equal(t, []string{"newest"}, messages(newer.Rows), "an authorized anchor must override a conflicting filter without repeating its newer neighbor")

		workerPage := mustPage(t, ctx, logs, logstore.PageQuery{Scope: logstore.Scope{ProjectID: project.ID, ClientID: workerID}, Limit: 10})
		worker := findMessage(t, workerPage.Rows, "worker")
		foreign, found, err := logs.Around(ctx, scope, worker.ID, 2, nil)
		require.NoError(t, err, "a foreign anchor lookup is a safe miss")
		require.False(t, found, "an anchor from another client must not resolve an instance")
		require.Empty(t, foreign.Rows, "a foreign anchor must never expose log context")
	})

	t.Run("date range excludes logs outside the selected day", func(t *testing.T) {
		start := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
		inside := logstore.TimeRange{From: start, To: start.AddDate(0, 0, 1)}
		page := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Limit: 10, Range: &inside})
		require.Equal(t, []string{"newest", "instance-api", "tie-high", "tie-low", "older"}, messages(page.Rows), "a UTC day includes every log written on that day")
		props := pages.LogsPageProps{ProjectId: project.ID.String(), ClientId: apiID.String(), Range: &pages.LogRange{
			TimeRange: inside, FromDate: "2026-10-14", ToDate: "2026-10-14",
		}}
		require.Contains(t, props.ToPath(nil, nil, "", ""), "from=2026-10-14", "pagination must keep the selected range")

		outside := logstore.TimeRange{From: inside.To, To: inside.To.AddDate(0, 0, 1)}
		empty := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Limit: 10, Range: &outside})
		require.Empty(t, empty.Rows, "logs from the previous UTC day stay outside the range")
	})

	t.Run("inactive client stays readable", func(t *testing.T) {
		inactiveProject, err := projectService.CreateProject(ctx, "LogsInactive", []string{"api"})
		require.NoError(t, err, "the scoped log operation must succeed")
		inactiveClient := clientByName(t, inactiveProject, "api")
		_, err = testutils.InsertLogFixtures(ctx, pool, []logstore.Row{{
			Scope: logstore.Scope{ClientID: inactiveClient}, Instance: "web", OccurredAt: base, Sequence: 1, Message: "kept",
		}})
		require.NoError(t, err, "the scoped log operation must succeed")
		_, err = projectService.UpdateProject(ctx, inactiveProject.ID, inactiveProject.Name, nil)
		require.NoError(t, err, "deactivate client while retaining its row")
		page := mustPage(t, ctx, logs, logstore.PageQuery{
			Scope: logstore.Scope{ProjectID: inactiveProject.ID, ClientID: inactiveClient},
			Limit: 10,
		})
		require.Equal(t, []string{"kept"}, messages(page.Rows), "deactivating a client must retain its logs")
		instances, err := logs.Instances(ctx, logstore.Scope{ProjectID: inactiveProject.ID, ClientID: inactiveClient})
		require.NoError(t, err, "the scoped log operation must succeed")
		require.Equal(t, []string{"web"}, instances, "instances of an inactive client stay available")
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

func mustPage(t *testing.T, ctx context.Context, logs *logstore.Store, req logstore.PageQuery) logstore.Page {
	t.Helper()
	page, err := logs.Page(ctx, req)
	require.NoError(t, err, "the scoped log operation must succeed")
	return page
}

func messages(logs []logstore.Row) []string {
	out := make([]string, len(logs))
	for i, log := range logs {
		out[i] = log.Message
	}
	return out
}

func findMessage(t *testing.T, logs []logstore.Row, message string) logstore.Row {
	t.Helper()
	for _, log := range logs {
		if log.Message == message {
			return log
		}
	}
	t.Fatalf("log %s not found", message)
	return logstore.Row{}
}
