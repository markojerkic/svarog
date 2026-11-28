package postgres_test

import (
	"context"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/markojerkic/svarog/internal/lib/logsearch"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/tests/testutils"
)

func TestSearchLogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool, nil)
	project, err := projectService.Create(ctx, projects.ProjectInput{Name: "LogsSearch", Clients: []string{"api", "worker"}})
	require.NoError(t, err, "create project and clients for log search")
	apiID := clientByName(t, project, "api")
	workerID := clientByName(t, project, "worker")
	logs := logstore.New(pool)
	base := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

	_, err = testutils.InsertLogFixtures(ctx, pool, []logstore.Row{
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base, Sequence: 1, Message: "older connection refused"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(5 * time.Second), Sequence: 2, Message: "foo only"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(10 * time.Second), Sequence: 5, Message: "needle low"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(10 * time.Second), Sequence: 5, Message: "needle high"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "api", OccurredAt: base.Add(20 * time.Second), Sequence: 8, Message: "connection timeout"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(30 * time.Second), Sequence: 9, Message: "Error code 500"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(40 * time.Second), Sequence: 10, Message: "foo-bar happened"},
		{Scope: logstore.Scope{ClientID: workerID}, Instance: "other-host", OccurredAt: base, Sequence: 1, Message: "connection refused elsewhere"},
		{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(50 * time.Second), Sequence: 11, Message: "SocketTimeoutException id=1 Čekanje"},
	})
	require.NoError(t, err, "insert searchable logs")

	scope := logstore.Scope{ProjectID: project.ID, ClientID: apiID}

	t.Run("Page returns the first search page", func(t *testing.T) {
		instance := []string{"web", "api"}
		page, err := logs.Page(ctx, logstore.PageQuery{
			Scope: scope, Search: parsedSearch(t, "connection"),
			Instances: instance, Limit: 1,
		})
		require.NoError(t, err, "search through Page must succeed")
		require.Equal(t, []string{"connection timeout"}, messages(page.Rows), "the first search page must contain only the newest matching log within the page limit")
		require.NotNil(t, page.Older, "search must reuse the existing page model")
		lastPage, err := logs.Page(ctx, logstore.PageQuery{
			Scope: scope, Search: parsedSearch(t, "connection"), Instances: instance, Limit: 3,
		})
		require.NoError(t, err, "a partial search page must load successfully")
		require.Len(t, lastPage.Rows, 2, "the partial page must contain both matching logs")
		require.Nil(t, lastPage.Older, "a partial page must not offer another older page")
	})

	t.Run("a time range limits matches", func(t *testing.T) {
		window := &logstore.TimeRange{From: base.Add(15 * time.Second), To: base.Add(time.Minute)}
		page := mustPage(t, ctx, logs, logstore.PageQuery{
			Scope: scope, Search: parsedSearch(t, "connection"), Range: window, Limit: 10,
		})
		require.Equal(t, []string{"connection timeout"}, messages(page.Rows), "a bounded search must ignore matches outside the window")
	})

	t.Run("both words are required and other clients stay hidden", func(t *testing.T) {
		found := mustSearch(t, ctx, logs, "connection refused", scope, nil, 10, nil, nil)
		require.Equal(t, []string{"older connection refused"}, messages(found), "search requires every word and ignores another client's match")
		oneWord := mustSearch(t, ctx, logs, "connection", scope, nil, 10, nil, nil)
		require.Equal(t, []string{"connection timeout", "older connection refused"}, messages(oneWord), "a single word matches every client log containing it, newest first")
		require.Equal(t, []string{"Error code 500"}, messages(mustSearch(t, ctx, logs, "error", scope, nil, 10, nil, nil)), "simple text search is case insensitive")
		require.Equal(t, []string{"Error code 500"}, messages(mustSearch(t, ctx, logs, "err", scope, nil, 10, nil, nil)), "a prefix must match inside a word")
		require.Equal(t, []string{"connection timeout"}, messages(mustSearch(t, ctx, logs, "time CONNECT", scope, nil, 10, nil, nil)), "all substrings must match regardless of order and case")
		require.Equal(t, []string{"SocketTimeoutException id=1 Čekanje"}, messages(mustSearch(t, ctx, logs, "cept id=1 čEK", scope, nil, 10, nil, nil)), "interior substrings, short extra terms and Croatian letters must match literally")
	})

	t.Run("hyphen is not an exclusion operator", func(t *testing.T) {
		require.Equal(t, []string{"foo-bar happened"}, messages(mustSearch(t, ctx, logs, "foo-bar", scope, nil, 10, nil, nil)), "a hyphenated query matches the log that contains both words")
		require.Equal(t, []string{"foo-bar happened", "foo only"}, messages(mustSearch(t, ctx, logs, "foo", scope, nil, 10, nil, nil)), "either side of a hyphen remains searchable")
		require.Equal(t, []string{"foo-bar happened"}, messages(mustSearch(t, ctx, logs, "bar", scope, nil, 10, nil, nil)), "the word after a hyphen is not treated as excluded")
	})

	t.Run("forward search returns the nearest matching rows", func(t *testing.T) {
		cursor := &logstore.Cursor{OccurredAt: base, Sequence: 0, ID: 1}
		page := mustSearch(t, ctx, logs, "foo", scope, nil, 1, nil, cursor)
		require.Equal(t, []string{"foo only"}, messages(page), "forward search must not jump over a nearer match to the latest matching row")
	})

	t.Run("empty query and wrong scope return nothing", func(t *testing.T) {
		require.Empty(t, mustSearch(t, ctx, logs, "", scope, nil, 10, nil, nil), "an empty search matches no logs")
		require.Empty(t, mustSearch(t, ctx, logs, "   ", scope, nil, 10, nil, nil), "a blank search matches no logs")
		require.Empty(t, mustSearch(t, ctx, logs, "connection", logstore.Scope{ProjectID: uuid.New(), ClientID: apiID}, nil, 10, nil, nil), "a client from another project is not searchable")
		require.Empty(t, mustSearch(t, ctx, logs, "connection", logstore.Scope{ProjectID: project.ID}, nil, 10, nil, nil), "an invalid client id returns no logs")
	})

	t.Run("instance filter and cursor id", func(t *testing.T) {
		apiInstance := []string{"api"}
		require.Equal(t, []string{"connection timeout"}, messages(mustSearch(t, ctx, logs, "connection", scope, apiInstance, 10, nil, nil)), "instance filter keeps only the named instance")
		empty := []string{}
		require.Empty(t, mustSearch(t, ctx, logs, "connection", scope, empty, 10, nil, nil), "an empty instance list matches nothing")

		needles := mustSearch(t, ctx, logs, "needle", scope, nil, 1, nil, nil)
		require.Equal(t, []string{"needle high"}, messages(needles), "same timestamp and sequence sorts the higher id first")
		withID := mustSearch(t, ctx, logs, "needle", scope, nil, 1, &logstore.Cursor{
			OccurredAt: needles[0].OccurredAt,
			Sequence:   needles[0].Sequence,
			ID:         needles[0].ID,
		}, nil)
		require.Equal(t, []string{"needle low"}, messages(withID), "cursor id returns the other row with the same timestamp and sequence")
	})

	t.Run("sparse matches across pages in both directions", func(t *testing.T) {
		// All rows tie on time and sequence. Only their IDs distinguish them.
		at := base.Add(24 * time.Hour)
		fixtures := make([]logstore.Row, 40)
		for i := range fixtures {
			fixtures[i] = logstore.Row{Scope: scope, Instance: "web", OccurredAt: at, Message: "unrelated"}
		}
		for _, i := range []int{0, 1, 20, 39} {
			fixtures[i].Message = "sparse match"
		}
		fixtures[38].Message, fixtures[38].Instance = "sparse match", "worker"
		fixtures[37].Message = "sparse only"
		ids, err := testutils.InsertLogFixtures(ctx, pool, fixtures)
		require.NoError(t, err)

		query := logstore.PageQuery{Scope: scope, Search: parsedSearch(t, "sparse match"), Instances: []string{"web"}, Limit: 2}
		first := mustPage(t, ctx, logs, query)
		require.Len(t, first.Rows, 2)
		require.Equal(t, []int64{ids[39], ids[20]}, []int64{first.Rows[0].ID, first.Rows[1].ID})
		require.NotNil(t, first.Older)
		query.Older = first.Older
		second := mustPage(t, ctx, logs, query)
		require.Len(t, second.Rows, 2)
		require.Equal(t, []int64{ids[1], ids[0]}, []int64{second.Rows[0].ID, second.Rows[1].ID})

		query.Older = nil
		query.Newer = &logstore.Cursor{OccurredAt: at, ID: ids[0]}
		forward := mustPage(t, ctx, logs, query)
		require.Len(t, forward.Rows, 2)
		require.Equal(t, []int64{ids[20], ids[1]}, []int64{forward.Rows[0].ID, forward.Rows[1].ID})
	})

	t.Run("pagination links preserve microseconds and tied rows", func(t *testing.T) {
		at := base.Add(time.Minute)
		ids, err := testutils.InsertLogFixtures(ctx, pool, []logstore.Row{
			{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: at.Add(100 * time.Microsecond), Sequence: 1, Message: "pagingtoken oldest"},
			{Scope: logstore.Scope{ClientID: apiID}, Instance: "api", OccurredAt: at.Add(300 * time.Microsecond), Sequence: 2, Message: "pagingtoken tie low"},
			{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: at.Add(300 * time.Microsecond), Sequence: 2, Message: "pagingtoken tie high"},
			{Scope: logstore.Scope{ClientID: apiID}, Instance: "api", OccurredAt: at.Add(300 * time.Microsecond), Sequence: 3, Message: "pagingtoken higher sequence"},
			{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: at.Add(700 * time.Microsecond), Sequence: 1, Message: "pagingtoken newer"},
			{Scope: logstore.Scope{ClientID: apiID}, Instance: "api", OccurredAt: at.Add(900 * time.Microsecond), Sequence: 1, Message: "pagingtoken newest"},
			{Scope: logstore.Scope{ClientID: apiID}, Instance: "ignored", OccurredAt: at.Add(time.Millisecond), Sequence: 9, Message: "pagingtoken excluded instance"},
			{Scope: logstore.Scope{ClientID: workerID}, Instance: "web", OccurredAt: at.Add(time.Millisecond), Sequence: 9, Message: "pagingtoken excluded client"},
		})
		require.NoError(t, err, "insert log rows with sub-millisecond timestamps and tied sequence numbers")
		instances := []string{"web", "api"}
		req := logstore.PageQuery{Scope: scope, Search: parsedSearch(t, "pagingtoken"), Instances: instances, Limit: 1}
		var found []int64
		for i := 0; i < 10; i++ {
			page, err := logs.Page(ctx, req)
			require.NoError(t, err, "every search page must load successfully")
			if len(page.Rows) == 0 {
				require.Nil(t, page.Older, "the last empty page must stop pagination")
				break
			}
			found = append(found, page.Rows[0].ID)
			u, err := url.Parse((pages.LogsPageProps{ProjectId: scope.ProjectID.String(), ClientId: scope.ClientID.String()}).ToPath(&instances, page.Older, "backward", "pagingtoken"))
			require.NoError(t, err, "pagination must generate a valid URL")
			query := u.Query()
			require.Equal(t, "pagingtoken", query.Get("search"), "every page link must preserve the executed query")
			require.Equal(t, instances, query["instance"], "every page link must preserve all instance filters")
			require.False(t, query.Has("cursorTime"), "new page links must not use lossy millisecond timestamps")
			micros, err := strconv.ParseInt(query.Get("cursorTimeUs"), 10, 64)
			require.NoError(t, err, "page links must contain a microsecond timestamp")
			sequence, err := strconv.ParseInt(query.Get("cursorSequenceNumber"), 10, 64)
			require.NoError(t, err, "page links must contain the sequence number")
			id, err := strconv.ParseInt(query.Get("cursorId"), 10, 64)
			require.NoError(t, err, "page links must contain the unique row id")
			require.Equal(t, "backward", query.Get("direction"), "the loader must continue toward older results")
			req.Older = &logstore.Cursor{OccurredAt: time.UnixMicro(micros), Sequence: sequence, ID: id}
		}
		require.Equal(t, []int64{ids[5], ids[4], ids[3], ids[2], ids[1], ids[0]}, found, "URL round trips must return every matching row exactly once in timestamp, sequence and id order")
	})

	t.Run("inactive client stays searchable", func(t *testing.T) {
		inactiveProject, err := projectService.Create(ctx, projects.ProjectInput{Name: "LogsSearchInactive", Clients: []string{"api"}})
		require.NoError(t, err, "create project for inactive search")
		inactiveClient := clientByName(t, inactiveProject, "api")
		_, err = testutils.InsertLogFixtures(ctx, pool, []logstore.Row{{
			Scope: logstore.Scope{ClientID: inactiveClient}, Instance: "web", OccurredAt: base, Sequence: 1, Message: "kept secret",
		}})
		require.NoError(t, err, "insert a log before deactivating the client")
		_, err = projectService.Update(ctx, inactiveProject.ID, projects.ProjectInput{Name: inactiveProject.Name})
		require.NoError(t, err, "deactivate client while retaining its row")
		found := mustSearch(t, ctx, logs, "secret", logstore.Scope{ProjectID: inactiveProject.ID, ClientID: inactiveClient}, nil, 10, nil, nil)
		require.Equal(t, []string{"kept secret"}, messages(found), "deactivating a client does not hide its logs from search")
	})

	t.Run("punctuation is literal in primary and additional terms", func(t *testing.T) {
		literal := `literal 100% foo_bar C:\logs\app "quoted" key=value -danger`
		_, err := testutils.InsertLogFixtures(ctx, pool, []logstore.Row{
			{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(time.Hour), Message: literal},
			{Scope: logstore.Scope{ClientID: apiID}, Instance: "web", OccurredAt: base.Add(2 * time.Hour), Message: `literal 1000 fooXbar C:logsapp unquoted key:value danger`},
			{Scope: logstore.Scope{ClientID: workerID}, Instance: "other-host", OccurredAt: base.Add(3 * time.Hour), Message: literal},
		})
		require.NoError(t, err, "insert literal-search logs and lookalikes in both clients")
		for _, query := range []string{`100%`, `foo_bar`, `C:\logs\app`, `"quoted"`, `key=value`, `-danger`, `literal 100%`, `literal foo_bar`, `literal C:\logs\app`} {
			t.Run(query, func(t *testing.T) {
				found := mustSearch(t, ctx, logs, query, scope, nil, 10, nil, nil)
				require.Equal(t, []string{literal}, messages(found), "punctuation in %q must not become wildcards, operators or token separators", query)
			})
		}
	})
}

func mustSearch(t *testing.T, ctx context.Context, logs *logstore.Store, search string, scope logstore.Scope, instances []string, limit int, older, newer *logstore.Cursor) []logstore.Row {
	t.Helper()
	page := mustPage(t, ctx, logs, logstore.PageQuery{Scope: scope, Instances: instances, Limit: limit, Older: older, Newer: newer, Search: parsedSearch(t, search)})
	return page.Rows
}

func parsedSearch(t *testing.T, text string) *logsearch.Query {
	t.Helper()
	query, err := logsearch.Parse(text)
	require.NoError(t, err, "the test query %q must be valid", text)
	return &query
}
