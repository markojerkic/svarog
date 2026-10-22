package postgres_test

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/lib/projects"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/server/db"
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
	var input []store.LogInsert
	for i := 0; i <= 10; i++ {
		input = append(input,
			store.LogInsert{ClientID: clientID, InstanceName: "api-1", OccurredAt: base.Add(time.Duration(i) * time.Second), SequenceNumber: 1, Message: fmt.Sprintf("api-%02d", i)},
			store.LogInsert{ClientID: clientID, InstanceName: "api-2", OccurredAt: base.Add(time.Duration(i)*time.Second + 500*time.Millisecond), SequenceNumber: 1, Message: fmt.Sprintf("other-%02d", i)},
		)
	}
	ids, err := store.InsertLogs(ctx, pool, input)
	require.NoError(t, err, "context fixtures must interleave two instances around the anchor")
	anchorID := ids[10]
	anchor := strconv.FormatInt(anchorID, 10)
	logs := db.NewLogService(pool, nil)
	instances := []string{"api-1"}
	request := db.LogPageRequest{ProjectId: project.ID.String(), ClientId: clientID.String(), PageSize: 2, LogLineId: &anchor}

	t.Run("anchor determines the instance", func(t *testing.T) {
		page, err := logs.GetLogs(ctx, request)
		require.NoError(t, err, "opening a valid context must succeed")
		require.NotEmpty(t, page.Logs, "context must contain the selected log")
		for _, log := range page.Logs {
			require.Equal(t, "api-1", log.Client.InstanceId, "context must exclude other instances even without an explicit instance parameter")
		}
	})

	t.Run("one page on each side of the anchor", func(t *testing.T) {
		req := request
		req.Instances = &instances
		page, err := logs.GetLogs(ctx, req)
		require.NoError(t, err, "context with an explicit instance must succeed")
		require.Equal(t, []string{"api-07", "api-06", "api-05", "api-04", "api-03"}, messages(page.Logs), "context must contain the anchor once and the nearest page of newer and older rows")
	})

	t.Run("both pagination directions are available", func(t *testing.T) {
		req := request
		req.Instances = &instances
		page, err := logs.GetLogs(ctx, req)
		require.NoError(t, err, "context must load before offering pagination")
		require.NotNil(t, page.BackwardCursor, "older rows beyond the context window must be reachable")
		require.NotNil(t, page.ForwardCursor, "newer rows beyond the context window must be reachable")
	})

	t.Run("forward pagination returns adjacent rows without gaps", func(t *testing.T) {
		page, err := logs.GetLogs(ctx, db.LogPageRequest{
			ProjectId: request.ProjectId,
			ClientId:  request.ClientId,
			Instances: &instances,
			PageSize:  2,
			Cursor:    &db.LastCursor{Timestamp: base.Add(5 * time.Second), SequenceNumber: 1, ID: anchorID},
		})
		require.NoError(t, err, "loading newer context rows must succeed")
		require.Equal(t, []string{"api-07", "api-06"}, messages(page.Logs), "forward pagination must return the nearest newer rows, not jump to the latest page")
	})

	t.Run("both directions reach every instance row exactly once", func(t *testing.T) {
		initial, err := logs.GetLogs(ctx, request)
		require.NoError(t, err, "context without an explicit instance must load")
		require.Equal(t, &instances, initial.Instances, "context must expose its resolved instance for pagination links")
		var found []int64
		for _, log := range initial.Logs {
			found = append(found, log.ID)
		}
		for _, direction := range []string{"backward", "forward"} {
			cursor := initial.BackwardCursor
			if direction == "forward" {
				cursor = initial.ForwardCursor
			}
			for i := 0; cursor != nil && i < 10; i++ {
				path, err := url.Parse(initial.ToPath(request.ProjectId, request.ClientId, initial.Instances, cursor, direction, ""))
				require.NoError(t, err, "context pagination must generate a valid URL")
				selected := path.Query()["instance"]
				page, err := logs.GetLogs(ctx, db.LogPageRequest{
					ProjectId: request.ProjectId, ClientId: request.ClientId,
					PageSize: request.PageSize, Instances: &selected, Cursor: cursor,
				})
				require.NoError(t, err, "context pagination without the original anchor must load")
				for _, log := range page.Logs {
					require.Equal(t, "api-1", log.Client.InstanceId, "every continuation must retain the anchor instance")
					found = append(found, log.ID)
				}
				cursor = page.BackwardCursor
				if direction == "forward" {
					cursor = page.ForwardCursor
				}
			}
			require.Nil(t, cursor, "pagination must terminate after reaching the end of its instance")
		}
		var expected []int64
		for i := 0; i < len(ids); i += 2 {
			expected = append(expected, ids[i])
		}
		require.ElementsMatch(t, expected, found, "both directions together must return all instance rows exactly once, including the anchor")
	})

	t.Run("context at instance boundaries", func(t *testing.T) {
		for _, index := range []int{0, 10} {
			id := strconv.FormatInt(ids[index*2], 10)
			req := request
			req.LogLineId = &id
			page, err := logs.GetLogs(ctx, req)
			require.NoError(t, err, "context must load at either end of an instance")
			require.Len(t, page.Logs, 3, "a boundary must show the anchor and the available page on its other side")
			if index == 0 {
				require.Equal(t, []string{"api-02", "api-01", "api-00"}, messages(page.Logs), "the oldest anchor must retain its newer neighbors")
				require.Nil(t, page.BackwardCursor, "the oldest anchor must not offer nonexistent older logs")
			} else {
				require.Equal(t, []string{"api-10", "api-09", "api-08"}, messages(page.Logs), "the newest anchor must retain its older neighbors")
				require.Nil(t, page.ForwardCursor, "the newest anchor must not offer nonexistent newer logs")
			}
		}
	})
}
