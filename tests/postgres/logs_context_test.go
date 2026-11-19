package postgres_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestLogContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := testutils.NewPostgresPool(t, ctx)
	project, err := projects.NewPostgresProjectsService(pool).CreateProject(ctx, "LogContext", []string{"api"})
	require.NoError(t, err, "context fixtures must have a project and client")
	clientID := project.Clients[0].ID
	base := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	var input []logstore.Row
	for i := 0; i <= 10; i++ {
		input = append(input,
			logstore.Row{Scope: logstore.Scope{ClientID: clientID}, Instance: "api-1", OccurredAt: base.Add(time.Duration(i) * time.Second), Sequence: 1, Message: fmt.Sprintf("api-%02d", i)},
			logstore.Row{Scope: logstore.Scope{ClientID: clientID}, Instance: "api-2", OccurredAt: base.Add(time.Duration(i)*time.Second + 500*time.Millisecond), Sequence: 1, Message: fmt.Sprintf("other-%02d", i)},
		)
	}
	ids, err := testutils.InsertLogFixtures(ctx, pool, input)
	require.NoError(t, err, "context fixtures must interleave two instances around the anchor")
	anchorID := ids[10]
	scope := logstore.Scope{ProjectID: project.ID, ClientID: clientID}
	logs := logstore.New(pool)
	instances := []string{"api-1"}
	const limit = 2

	t.Run("anchor determines the instance", func(t *testing.T) {
		page, found, err := logs.Around(ctx, scope, anchorID, limit, nil)
		require.NoError(t, err, "opening a valid context must succeed")
		require.True(t, found, "the requested anchor must belong to the client")
		require.NotEmpty(t, page.Rows, "context must contain the selected log")
		for _, log := range page.Rows {
			require.Equal(t, "api-1", log.Instance, "context must exclude other instances even without an explicit instance parameter")
		}
	})

	t.Run("one page on each side of the anchor", func(t *testing.T) {
		page, found, err := logs.Around(ctx, scope, anchorID, limit, nil)
		require.NoError(t, err, "context must load neighbors around the anchor")
		require.True(t, found, "the requested anchor must belong to the client")
		require.Equal(t, []string{"api-07", "api-06", "api-05", "api-04", "api-03"}, messages(page.Rows), "context must contain the anchor once and the nearest page of newer and older rows")
	})

	t.Run("both pagination directions are available", func(t *testing.T) {
		page, found, err := logs.Around(ctx, scope, anchorID, limit, nil)
		require.NoError(t, err, "context must load before offering pagination")
		require.True(t, found, "the requested anchor must belong to the client")
		require.NotNil(t, page.Older, "older rows beyond the context window must be reachable")
		require.NotNil(t, page.Newer, "newer rows beyond the context window must be reachable")
	})

	t.Run("forward pagination returns adjacent rows without gaps", func(t *testing.T) {
		page, err := logs.Page(ctx, logstore.PageQuery{
			Scope: scope, Instances: instances, Limit: limit,
			Newer: &logstore.Cursor{OccurredAt: base.Add(5 * time.Second), Sequence: 1, ID: anchorID},
		})
		require.NoError(t, err, "loading newer context rows must succeed")
		require.Equal(t, []string{"api-07", "api-06"}, messages(page.Rows), "forward pagination must return the nearest newer rows, not jump to the latest page")
	})

	t.Run("both directions reach every instance row exactly once", func(t *testing.T) {
		initial, found, err := logs.Around(ctx, scope, anchorID, limit, nil)
		require.NoError(t, err, "context without an explicit instance must load")
		require.True(t, found, "the requested anchor must belong to the client")
		require.NotNil(t, initial.AnchorInstance, "the initial context must resolve the instance")
		require.Equal(t, instances[0], *initial.AnchorInstance, "context must expose its resolved instance for pagination links")
		var seen []int64
		for _, log := range initial.Rows {
			seen = append(seen, log.ID)
		}
		for _, direction := range []string{"backward", "forward"} {
			cursor := initial.Older
			if direction == "forward" {
				cursor = initial.Newer
			}
			for i := 0; cursor != nil && i < 10; i++ {
				path, err := url.Parse((pages.LogsPageProps{ProjectId: scope.ProjectID.String(), ClientId: scope.ClientID.String()}).ToPath(&instances, cursor, direction, ""))
				require.NoError(t, err, "context pagination must generate a valid URL")
				selected := path.Query()["instance"]
				query := logstore.PageQuery{Scope: scope, Limit: limit, Instances: selected}
				if direction == "backward" {
					query.Older = cursor
				} else {
					query.Newer = cursor
				}
				page, err := logs.Page(ctx, query)
				require.NoError(t, err, "context pagination without the original anchor must load")
				for _, log := range page.Rows {
					require.Equal(t, "api-1", log.Instance, "every continuation must retain the anchor instance")
					seen = append(seen, log.ID)
				}
				cursor = page.Older
				if direction == "forward" {
					cursor = page.Newer
				}
			}
			require.Nil(t, cursor, "pagination must terminate after reaching the end of its instance")
		}
		var expected []int64
		for i := 0; i < len(ids); i += 2 {
			expected = append(expected, ids[i])
		}
		require.ElementsMatch(t, expected, seen, "both directions together must return all instance rows exactly once, including the anchor")
	})

	t.Run("context at instance boundaries", func(t *testing.T) {
		for _, index := range []int{0, 10} {
			page, found, err := logs.Around(ctx, scope, ids[index*2], limit, nil)
			require.NoError(t, err, "context must load at either end of an instance")
			require.True(t, found, "the requested anchor must belong to the client")
			require.Len(t, page.Rows, 3, "a boundary must show the anchor and the available page on its other side")
			if index == 0 {
				require.Equal(t, []string{"api-02", "api-01", "api-00"}, messages(page.Rows), "the oldest anchor must retain its newer neighbors")
				require.Nil(t, page.Older, "the oldest anchor must not offer nonexistent older logs")
			} else {
				require.Equal(t, []string{"api-10", "api-09", "api-08"}, messages(page.Rows), "the newest anchor must retain its older neighbors")
				require.Nil(t, page.Newer, "the newest anchor must not offer nonexistent newer logs")
			}
		}
	})
}
