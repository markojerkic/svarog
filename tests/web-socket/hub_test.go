package websocket

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/server/types"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestWatchHubSuite(t *testing.T) {
	suite.Run(t, new(WatchHubSuite))
}

func (s *WatchHubSuite) subscribe(projectID, clientID string) *websocket.LogSubscription {
	s.T().Helper()
	sub, err := s.watchHub.Subscribe(projectID, clientID)
	require.NoError(s.T(), err, "the viewer should subscribe to its log scope")
	s.T().Cleanup(sub.Close)
	return sub
}

func receiveLog(t *testing.T, sub *websocket.LogSubscription) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	message, err := sub.Next(ctx)
	require.NoError(t, err, "a subscribed viewer should receive its published batch")
	return string(message)
}

func (s *WatchHubSuite) TestScopeIsolationAndFanout() {
	t := s.T()
	first := s.subscribe("project-1", "client-1")
	second := s.subscribe("project-1", "client-1")
	otherClient := s.subscribe("project-1", "client-2")
	otherProject := s.subscribe("project-2", "client-1")

	require.NoError(t, s.watchHub.SendLogLine("project-1", "client-1", []byte("shared")), "publishing should succeed")
	require.Equal(t, "shared", receiveLog(t, first), "the first viewer should receive the shared fragment")
	require.Equal(t, "shared", receiveLog(t, second), "the second viewer should receive the same fragment")
	for _, sub := range []*websocket.LogSubscription{otherClient, otherProject} {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, err := sub.Next(ctx)
		cancel()
		require.ErrorIs(t, err, context.DeadlineExceeded, "another client or project must not receive the fragment")
	}
}

func (s *WatchHubSuite) TestSlowViewerAndConcurrentClose() {
	t := s.T()
	slow := s.subscribe("pressure", "slow")
	healthy := s.subscribe("pressure", "healthy")
	large := []byte(strings.Repeat("x", 600*1024))
	for range 3 {
		require.NoError(t, s.watchHub.SendLogLine("pressure", "slow", large), "large frames below the broker limit should publish")
	}
	require.NoError(t, s.watchHub.SendLogLine("pressure", "healthy", []byte("still live")), "other viewers should remain writable")
	require.Equal(t, "still live", receiveLog(t, healthy), "a slow viewer must not stall another viewer")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, err := slow.Next(ctx)
		if err != nil {
			require.ErrorIs(t, err, websocket.ErrLiveResync, "a viewer that exceeded its byte budget must require refresh, not silently lose rows")
			break
		}
	}

	closing := s.subscribe("pressure", "closing")
	var publishers sync.WaitGroup
	publishers.Add(1)
	go func() {
		defer publishers.Done()
		for range 500 {
			s.watchHub.SendLogLine("pressure", "closing", []byte("in flight"))
		}
	}()
	closing.Close()
	closing.Close()
	publishers.Wait()
	_, err := closing.Next(ctx)
	require.ErrorIs(t, err, context.Canceled, "closing with in-flight callbacks must be safe and idempotent")
}

func (s *WatchHubSuite) TestCommittedBatchesAreBoundedAndScoped() {
	t := s.T()
	first := s.subscribe("render", "first")
	second := s.subscribe("render", "second")
	at := time.Now().Truncate(time.Microsecond).Add(123 * time.Nanosecond)
	rows := make([]types.StoredLog, 206)
	for i := range rows {
		rows[i] = types.StoredLog{
			ID: int64(i + 1), Timestamp: at, SequenceNumber: i,
			LogLine: fmt.Sprintf("row-%d <script> %s", i, strings.Repeat("x", 1024)),
			Client:  types.StoredClient{ProjectId: "render", ClientId: "first", InstanceId: "host"},
		}
	}
	rows[len(rows)-1].Client.ClientId = "second"
	s.WsLogRenderer.RenderBatch(context.Background(), rows)
	ids := make(map[string]bool)
	idPattern := regexp.MustCompile(`<pre[^>]*data-log-id="(\d+)"`)
	frames := 0
	for len(ids) < 205 {
		frame := receiveLog(t, first)
		frames++
		require.LessOrEqual(t, len(frame), 128*1024, "a rendered batch must respect its encoded byte limit")
		require.LessOrEqual(t, strings.Count(frame, "<pre"), 100, "a batch must contain at most 100 rows")
		require.Equal(t, 1, strings.Count(frame, "hx-swap-oob="), "rows in a batch should share one OOB wrapper")
		require.NotContains(t, frame, "<script>", "log messages must stay HTML escaped inside batch fragments")
		require.Contains(t, frame, `data-timestamp="`+strconv.FormatInt(at.Truncate(time.Microsecond).UnixNano(), 10)+`"`, "live timestamps should have the same precision as stored logs")
		for _, match := range idPattern.FindAllStringSubmatch(frame, -1) {
			require.False(t, ids[match[1]], "each committed row must be rendered only once")
			ids[match[1]] = true
		}
	}
	require.Greater(t, frames, 1, "a large committed batch should be split into several bounded messages")
	require.False(t, ids["206"], "another client's row must not leak into the first client's batch")
	require.Contains(t, receiveLog(t, second), `data-log-id="206"`, "the second client should receive its own row")

	oversized := rows[0]
	oversized.LogLine = strings.Repeat("<", 64*1024)
	s.WsLogRenderer.RenderBatch(context.Background(), []types.StoredLog{oversized})
	require.Contains(t, receiveLog(t, first), "data-log-resync", "a row that exceeds the encoded payload limit must visibly invalidate the view instead of silently disappearing")
}
