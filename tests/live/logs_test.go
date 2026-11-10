package live_test

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/server/live"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type LiveSuite struct{ testutils.BaseSuite }

func TestLiveSuite(t *testing.T) { suite.Run(t, new(LiveSuite)) }

func (s *LiveSuite) gateway() (*live.Logs, *nats.Conn) {
	s.T().Helper()
	cfg := testutils.DefaultBaseSuiteConfig()
	conn, err := nats.Connect(s.NatsAddr, nats.UserJWTAndSeed(cfg.NatsJwt, cfg.NatsSeed), nats.ReconnectBufSize(-1))
	require.NoError(s.T(), err, "a gateway replica must connect independently of the writer")
	gateway := live.New(conn)
	s.T().Cleanup(func() { gateway.Close(); conn.Close() })
	return gateway, conn
}

func open(t *testing.T, logs *live.Logs, scope live.Scope, instances ...string) *live.Session {
	t.Helper()
	viewer, err := logs.Open(context.Background(), scope, live.Selector{Instances: instances})
	require.NoError(t, err, "the authorized viewer must open after its broker subscriptions are ready")
	t.Cleanup(viewer.Close)
	return viewer
}

func receive(t *testing.T, viewer *live.Session) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	message, err := viewer.Next(ctx)
	require.NoError(t, err, "an interested viewer must receive the committed HTML batch")
	return string(message)
}

func row(scope live.Scope, instance string, id int64, message string) types.StoredLog {
	return types.StoredLog{ID: id, Timestamp: time.Now(), SequenceNumber: int(id), LogLine: message,
		Client: types.StoredClient{ProjectId: scope.ProjectID.String(), ClientId: scope.ClientID.String(), InstanceId: instance}}
}

func newScope() live.Scope { return live.Scope{ProjectID: uuid.New(), ClientID: uuid.New()} }

func subscriptions(t *testing.T, conn *nats.Conn, count int) {
	t.Helper()
	require.Eventually(t, func() bool { return conn.NumSubscriptions() == count }, 6*time.Second, 10*time.Millisecond,
		"broker interest must be shared and reduced to the current local source union")
}

func (s *LiveSuite) TestReplicaRoutingAndSharedInterest() {
	t := s.T()
	first, conn := s.gateway()
	second, _ := s.gateway()
	scope := newScope()
	otherScope := live.Scope{ProjectID: uuid.New(), ClientID: scope.ClientID}
	instance := "host.a * > / č"
	a := open(t, first, scope, instance)
	b := open(t, first, scope, instance)
	subscriptions(t, conn, 2) // exact source plus client invalidation
	c := open(t, first, scope, "")
	subscriptions(t, conn, 3)
	remote := open(t, second, scope, instance, "")
	foreign := open(t, second, otherScope)
	s.LiveLogs.AfterCommit(context.Background(), []types.StoredLog{
		row(scope, instance, 1, "selected source"), row(scope, "", 2, "unnamed source"),
		row(scope, "not-selected", 3, "must not leak"), row(otherScope, instance, 4, "other project"),
	})
	require.Contains(t, receive(t, a), "selected source", "the writer must publish even without local viewers")
	require.Contains(t, receive(t, b), "selected source", "tabs sharing a source must both receive it")
	require.Contains(t, receive(t, c), "unnamed source", "an empty name is an exact instance, not the all-instances selector")
	remoteFrames := receive(t, remote) + receive(t, remote)
	require.Contains(t, remoteFrames, "selected source", "ordinary subscriptions must fan out to every interested replica")
	require.Contains(t, remoteFrames, "unnamed source", "a multi-instance viewer must receive its selected union")
	require.NotContains(t, remoteFrames, "must not leak", "unselected instances must be filtered before rendering and delivery")
	require.NotContains(t, remoteFrames, "other project", "the same client ID in another project must stay isolated")
	require.Contains(t, receive(t, foreign), "other project", "the other project must receive its own events")
	for _, viewer := range []*live.Session{a, b, c, remote, foreign} {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_, err := viewer.Next(ctx)
		cancel()
		require.ErrorIs(t, err, context.DeadlineExceeded, "viewers must not receive another scope or instance's remaining events")
	}
	all := open(t, first, scope)
	subscriptions(t, conn, 1)
	s.LiveLogs.AfterCommit(context.Background(), []types.StoredLog{row(scope, "new-host", 5, "wildcard source")})
	require.Contains(t, receive(t, all), "wildcard source", "an all-instances view must include newly appearing instances")
	all.Close()
	subscriptions(t, conn, 3)
	a.Close()
	b.Close()
	subscriptions(t, conn, 2)
	c.Close()
	subscriptions(t, conn, 0)
}

func (s *LiveSuite) TestSubscriptionHandoverDoesNotLoseOrDuplicateRows() {
	t := s.T()
	gateway, conn := s.gateway()
	scope := newScope()
	viewer := open(t, gateway, scope, "host")
	var publisher sync.WaitGroup
	publisher.Go(func() {
		for i := int64(1); i <= 60; i++ {
			s.LiveLogs.AfterCommit(context.Background(), []types.StoredLog{row(scope, "host", i, fmt.Sprintf("message-%d", i))})
			runtime.Gosched()
		}
	})
	for range 3 {
		all := open(t, gateway, scope)
		all.Close()
		subscriptions(t, conn, 2)
	}
	var tabs sync.WaitGroup
	openErrors := make(chan error, 8)
	for i := range 8 {
		tabs.Go(func() {
			selector := live.Selector{}
			if i%2 == 0 {
				selector.Instances = []string{"host"}
			}
			view, err := gateway.Open(context.Background(), scope, selector)
			if err == nil {
				view.Close()
				view.Close()
			}
			openErrors <- err
		})
	}
	tabs.Wait()
	close(openErrors)
	for err := range openErrors {
		require.NoError(t, err, "concurrent tabs must safely share and release the same client interest")
	}
	subscriptions(t, conn, 2)
	publisher.Wait()
	ids := make(map[string]bool)
	for range 60 {
		frame := receive(t, viewer)
		id := strings.SplitN(strings.SplitN(frame, `data-log-id="`, 2)[1], `"`, 2)[0]
		require.False(t, ids[id], "overlapping wildcard and exact subscriptions must not render a publication twice")
		ids[id] = true
	}
	require.Len(t, ids, 60, "subscribing new coverage before draining old coverage must retain every live row")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := viewer.Next(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded, "no duplicate publication may remain after the handover")
}

func (s *LiveSuite) TestStructuredAndRenderedBatchBudgets() {
	t := s.T()
	gateway, _ := s.gateway()
	scope := newScope()
	viewer := open(t, gateway, scope)
	wire, err := s.NatsConn.Conn.SubscribeSync("committed.logs." + scope.ProjectID.String() + "." + scope.ClientID.String() + ".*")
	require.NoError(t, err, "the test must observe the internal wire protocol independently")
	t.Cleanup(func() { wire.Unsubscribe() })
	require.NoError(t, s.NatsConn.Conn.FlushTimeout(time.Second), "wire observation must be active before publishing")
	at := time.Now().Truncate(time.Microsecond).Add(123 * time.Nanosecond)
	rows := make([]types.StoredLog, 205)
	for i := range rows {
		rows[i] = row(scope, "host", int64(i+1), "<script>"+strings.Repeat("x", 1024))
		rows[i].Timestamp = at
	}
	s.LiveLogs.AfterCommit(context.Background(), rows)
	count := 0
	for count < len(rows) {
		message, err := wire.NextMsg(5 * time.Second)
		require.NoError(t, err, "committed structured batches must be published without local interest")
		require.LessOrEqual(t, len(message.Data), 128*1024, "structured payloads must stay below their encoded byte limit")
		var batch struct {
			Version int `json:"version"`
			Rows    []struct {
				ID         string `json:"id"`
				OccurredUs string `json:"occurredUs"`
			} `json:"rows"`
		}
		require.NoError(t, json.Unmarshal(message.Data, &batch), "the broker must carry structured data, not HTML")
		require.Equal(t, 1, batch.Version, "the internal wire format must be explicitly versioned")
		require.LessOrEqual(t, len(batch.Rows), 100, "structured groups must also be bounded by row count")
		require.Equal(t, strconv.FormatInt(at.UnixMicro(), 10), batch.Rows[0].OccurredUs, "timestamps must match PostgreSQL microsecond precision")
		count += len(batch.Rows)
	}
	seen := make(map[string]bool)
	for len(seen) < len(rows) {
		frame := receive(t, viewer)
		require.LessOrEqual(t, len(frame), 128*1024, "HTML must respect its own encoded byte budget after rendering")
		require.LessOrEqual(t, strings.Count(frame, "<pre"), 100, "HTML groups must retain the row limit from phase A")
		require.Equal(t, 1, strings.Count(frame, "hx-swap-oob="), "rows in a frame must share one OOB wrapper")
		require.NotContains(t, frame, "<script>", "messages must be escaped when rendering the shared HTML")
		require.Contains(t, frame, `data-timestamp="`+strconv.FormatInt(at.Truncate(time.Microsecond).UnixNano(), 10)+`"`, "live ordering must use stored timestamp precision")
		for _, part := range strings.Split(frame, `data-log-id="`)[1:] {
			id := strings.SplitN(part, `"`, 2)[0]
			// Each row also has the existing menu button's ID attribute.
			seen[id] = true
		}
	}
	require.Len(t, seen, 205, "every row must survive splitting into both wire and HTML groups")
	for _, input := range []struct{ instance, message string }{
		{"host", strings.Repeat("<", 20*1024)},
		{"host", strings.Repeat("<", 64*1024)},
		{"host", strings.Repeat("x", 9*1024*1024)},
		{strings.Repeat("h", 4096), "oversized subject"},
	} {
		separate := newScope()
		oversized := open(t, gateway, separate, "host")
		s.LiveLogs.AfterCommit(context.Background(), []types.StoredLog{row(separate, input.instance, 1, input.message)})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := oversized.Next(ctx)
		cancel()
		require.ErrorIs(t, err, live.ErrLiveResync, "oversized HTML, structured payloads or subjects must invalidate the view without disconnecting the gateway")
	}
	s.LiveLogs.AfterCommit(context.Background(), []types.StoredLog{row(scope, "host", 206, "still connected")})
	require.Contains(t, receive(t, viewer), "still connected", "a rejected oversized subject must not break the broker connection or other client views")
}

func (s *LiveSuite) TestSlowViewerAndConcurrentClose() {
	t := s.T()
	gateway, _ := s.gateway()
	scope := newScope()
	slow := open(t, gateway, scope)
	healthy := open(t, gateway, scope)
	healthyResult := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for received := 0; received < 24; {
			frame, err := healthy.Next(ctx)
			if err != nil {
				healthyResult <- err
				return
			}
			received += strings.Count(string(frame), "<pre")
		}
		healthyResult <- nil
	}()
	rows := make([]types.StoredLog, 24)
	for i := range rows {
		rows[i] = row(scope, "host", int64(i+1), strings.Repeat("x", 35*1024))
	}
	s.LiveLogs.AfterCommit(context.Background(), rows)
	require.NoError(t, <-healthyResult, "a fast viewer of the same source must not be closed by another viewer's backpressure")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, err := slow.Next(ctx)
		if err != nil {
			require.ErrorIs(t, err, live.ErrLiveResync, "a viewer exceeding its queue byte limit must require refresh")
			break
		}
	}
	closing := open(t, gateway, newScope())
	closing.Close()
	closing.Close()
	_, err := closing.Next(ctx)
	require.ErrorIs(t, err, context.Canceled, "viewer closure must be immediate and idempotent")
	var publishers sync.WaitGroup
	publishers.Go(func() {
		for i := int64(25); i < 125; i++ {
			s.LiveLogs.AfterCommit(context.Background(), []types.StoredLog{row(scope, "host", i, "in flight")})
		}
	})
	healthy.Close()
	publishers.Wait()
}

func (s *LiveSuite) TestGatewayConnectionLossInvalidatesItsViewers() {
	t := s.T()
	gateway, conn := s.gateway()
	first := open(t, gateway, newScope())
	second := open(t, gateway, newScope(), "host")
	conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, viewer := range []*live.Session{first, second} {
		_, err := viewer.Next(ctx)
		require.ErrorIs(t, err, live.ErrLiveResync, "a shared gateway disconnect must invalidate every affected local view")
	}
}
