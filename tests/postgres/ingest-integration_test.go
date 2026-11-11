package postgres_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/ingest"
	"github.com/markojerkic/svarog/internal/server/live"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	natscontainer "github.com/testcontainers/testcontainers-go/modules/nats"
)

func TestPostgresClientsIngestIntoPostgresLogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)

	// Set up PostgreSQL projects with distinct client UUIDs and JetStream.
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool)
	first, err := projectService.CreateProject(ctx, "First", []string{"api"})
	require.NoError(t, err, "create first PG project with active client")
	second, err := projectService.CreateProject(ctx, "Second", []string{"api"})
	require.NoError(t, err, "create second PG project with same client name and a distinct UUID")

	natsContainer, err := natscontainer.Run(ctx, "nats:2.15.1-alpine",
		natscontainer.WithConfigFile(strings.NewReader("jetstream { store_dir: /data }")),
	)
	if natsContainer != nil {
		testcontainers.CleanupContainer(t, natsContainer)
	}
	require.NoError(t, err, "start JetStream for UUID ingestion integration test")
	natsURL, err := natsContainer.ConnectionString(ctx)
	require.NoError(t, err, "get JetStream connection string")
	conn, err := natsconn.NewNatsConnection(natsconn.NatsConnectionConfig{
		NatsAddr:        natsURL,
		EnableJetStream: true,
	})
	require.NoError(t, err, "connect to JetStream and create log stream")
	t.Cleanup(conn.Close)

	liveLogs := live.New(conn.Conn)
	t.Cleanup(liveLogs.Close)
	logService := db.NewLogService(pool, liveLogs)
	ingestService, err := ingest.Open(ctx, ingest.Options{Connection: conn, Pool: pool, Writer: logService})
	require.NoError(t, err, "prepare durable log ingestion before publishing")
	runCtx, stop := context.WithCancel(ctx)
	ingestDone := make(chan error, 1)
	go func() { ingestDone <- ingestService.Run(runCtx) }()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, ingestService.Shutdown(shutdownCtx), "ingest drains before connections close")
		stop()
		select {
		case err := <-ingestDone:
			require.NoError(t, err, "ingest loop stops without error")
		case <-time.After(5 * time.Second):
			t.Error("ingest loop did not stop")
		}
	})

	publish := func(projectID, clientID, message string) {
		t.Helper()
		payload, err := json.Marshal(rpc.LogLine{
			Message:    message,
			Timestamp:  time.Now().UTC(),
			InstanceId: "instance",
			Sequence:   1,
		})
		require.NoError(t, err, "encode valid RPC log line")
		_, err = conn.JetStream.Publish(ctx, fmt.Sprintf("logs.%s.%s", projectID, clientID), payload)
		require.NoError(t, err, "publish log for UUID project %s and client %s", projectID, clientID)
	}
	waitForMessage := func(clientID uuid.UUID, message string) {
		t.Helper()
		require.Eventually(t, func() bool {
			var found bool
			err := pool.QueryRow(ctx, "SELECT true FROM logs WHERE client_id = $1 AND message = $2", clientID, message).Scan(&found)
			return err == nil && found
		}, 15*time.Second, 50*time.Millisecond, "accepted log %q must reach PostgreSQL for client %s", message, clientID)
	}

	// Valid project/client UUID pairs are stored in PostgreSQL.
	publish(first.ID.String(), first.Clients[0].ID.String(), "first accepted")
	publish(second.ID.String(), second.Clients[0].ID.String(), "second accepted")

	waitForMessage(first.Clients[0].ID, "first accepted")
	waitForMessage(second.Clients[0].ID, "second accepted")

	// A temporary DB validation failure retries the message until the lock is released.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err, "begin temporary database lock to simulate unavailable client validation")
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, "LOCK TABLE clients IN ACCESS EXCLUSIVE MODE")
	require.NoError(t, err, "block client validation until its query timeout")

	publish(first.ID.String(), first.Clients[0].ID.String(), "retry accepted")
	consumer, err := conn.JetStream.Consumer(ctx, "LOGS", "log-processor")
	require.NoError(t, err, "load ingest consumer to observe retries")

	require.Eventually(t, func() bool {
		info, err := consumer.Info(ctx)
		return err == nil && info.NumRedelivered > 0
	}, 15*time.Second, 50*time.Millisecond, "database validation timeout must retry the message rather than acknowledge or discard it")

	require.NoError(t, tx.Commit(ctx), "restore database validation before retry is processed")
	waitForMessage(first.Clients[0].ID, "retry accepted")

	// Inactive, foreign, unknown, legacy, and malformed messages must not be persisted.
	_, err = projectService.UpdateProject(ctx, first.ID, first.Name, nil)
	require.NoError(t, err, "deactivate first client while preserving its UUID")

	publish(first.ID.String(), first.Clients[0].ID.String(), "inactive rejected")
	publish(second.ID.String(), first.Clients[0].ID.String(), "foreign client rejected")
	publish(second.ID.String(), uuid.NewString(), "unknown client rejected")
	publish(uuid.NewString(), second.Clients[0].ID.String(), "unknown project rejected")
	publish("not-a-uuid", "client-name", "legacy ids rejected")
	validSubject := "logs." + second.ID.String() + "." + second.Clients[0].ID.String()
	for _, invalid := range []struct {
		subject string
		payload string
	}{
		{
			subject: "logs.invalid",
			payload: `{}`,
		},
		{
			subject: "logs.extra." + second.ID.String() + "." + second.Clients[0].ID.String(),
			payload: `{}`,
		},
		{
			subject: validSubject,
			payload: `{"sequence":-1}`,
		},
		{
			subject: validSubject,
			payload: `not json`,
		},
	} {
		_, err := conn.JetStream.Publish(ctx, invalid.subject, []byte(invalid.payload))
		require.NoError(t, err, "publish invalid message to ensure ingest terminates it safely")
	}

	// Wait for the valid barrier and all acknowledgements before checking rejection counts.
	publish(second.ID.String(), second.Clients[0].ID.String(), "validation barrier")

	waitForMessage(second.Clients[0].ID, "validation barrier")
	require.Eventually(t, func() bool {
		info, err := consumer.Info(ctx)
		return err == nil && info.NumPending == 0 && info.NumAckPending == 0
	}, 10*time.Second, 50*time.Millisecond, "permanently invalid messages must be terminated, not retried forever")

	var total int64
	err = pool.QueryRow(ctx, "SELECT count(*) FROM logs").Scan(&total)
	require.NoError(t, err, "count persisted PostgreSQL logs after validation barrier")
	require.EqualValues(t, 4, total, "inactive, foreign, missing and malformed client logs must not reach PostgreSQL; retried valid log is stored exactly once")

	// Deactivation preserves historical ownership but disallows new credentials.
	belongs, err := projectService.ClientBelongsToProject(ctx, first.ID.String(), first.Clients[0].ID.String())

	require.NoError(t, err, "check historical client ownership after deactivation")
	require.True(t, belongs, "inactive client still belongs to its project for historical log reads")
	require.False(t, projectService.ProjectExists(ctx, first.ID.String(), first.Clients[0].ID.String()), "inactive client cannot receive new credentials")

	// Reactivation keeps the client UUID and both original and retried history.
	_, err = projectService.UpdateProject(ctx, first.ID, first.Name, []string{"api"})
	require.NoError(t, err, "reactivate first client by name without changing its UUID")

	publish(first.ID.String(), first.Clients[0].ID.String(), "reactivated accepted")

	waitForMessage(first.Clients[0].ID, "reactivated accepted")
	var history int64
	err = pool.QueryRow(ctx, "SELECT count(*) FROM logs WHERE client_id = $1", first.Clients[0].ID).Scan(&history)

	require.NoError(t, err, "count PostgreSQL logs after UUID-preserving reactivation")
	require.EqualValues(t, 3, history, "reactivated client retains original and retried history and adds only the new accepted log")
}

func TestIngestCommitLifecycle(t *testing.T) {
	t.Run("retry stays unacknowledged and heartbeats protect work from another replica", func(t *testing.T) {
		env := newIngestFixture(t)
		entered, release := make(chan struct{}), make(chan struct{})
		var attempts atomic.Int32
		opts := env.options
		opts.AckWait = 300 * time.Millisecond
		opts.Writer = ingestWriterFunc(func(ctx context.Context, input []db.IngestLog) error {
			attempt := attempts.Add(1)
			if attempt == 1 {
				return &pgconn.PgError{Code: "40001"}
			}
			if attempt == 2 {
				close(entered)
			}
			select {
			case <-release:
				return env.writer.CommitLogs(ctx, input)
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		first := runIngest(t, env.ctx, opts)
		payload := publishIngestEvent(t, env, "kept until commit")
		select {
		case <-entered:
		case <-env.ctx.Done():
			t.Fatal("ingest must retry a transient failure")
		}
		consumer, err := env.conn.JetStream.Consumer(env.ctx, "LOGS", "log-processor")
		require.NoError(t, err, "load shared durable to observe pending work")
		info, err := consumer.Info(env.ctx)
		require.NoError(t, err, "read outstanding delivery before DB commit")
		require.Equal(t, 1, info.NumAckPending, "a message in a writer is not acknowledged before commit")
		secondOpts := env.options
		secondOpts.AckWait = opts.AckWait
		runIngest(t, env.ctx, secondOpts)
		require.Never(t, func() bool {
			var count int
			_ = env.pool.QueryRow(env.ctx, "SELECT count(*) FROM logs").Scan(&count)
			return count != 0
		}, 3*opts.AckWait, 40*time.Millisecond, "progress heartbeats prevent another replica from stealing a blocked delivery")
		shutdown := make(chan error, 1)
		go func() { shutdown <- first.service.Shutdown(env.ctx) }()
		close(release)
		require.NoError(t, <-shutdown, "graceful shutdown completes the active batch before returning")
		<-first.done
		require.NoError(t, first.err, "Run exits cleanly after a successful drain")
		require.Equal(t, int32(2), attempts.Load(), "a transient failure is retried without dropping the batch")
		_, err = env.conn.JetStream.Publish(env.ctx, env.subject, payload, jetstream.WithMsgID("different-header-same-event"))
		require.NoError(t, err, "republish original bytes with a different untrusted header ID")
		waitIngestEmpty(t, env)
		var count int
		require.NoError(t, env.pool.QueryRow(env.ctx, "SELECT count(*) FROM logs").Scan(&count), "read logs after both deliveries complete")
		require.Equal(t, 1, count, "server-side identity prevents duplicates across replicas and publisher headers")
	})

	t.Run("a lost commit result stops live views and retries without duplicates", func(t *testing.T) {
		env := newIngestFixture(t)
		liveConn, err := natsconn.NewNatsConnection(natsconn.NatsConnectionConfig{NatsAddr: env.conn.Conn.ConnectedUrl(), DisableReconnectBuffer: true})
		require.NoError(t, err, "open a dedicated live publication connection")
		t.Cleanup(liveConn.Close)
		liveLogs := live.New(liveConn.Conn)
		t.Cleanup(liveLogs.Close)
		viewer, err := liveLogs.Open(env.ctx, env.scope, live.Selector{})
		require.NoError(t, err, "open a live view before the event is committed")
		t.Cleanup(viewer.Close)

		var arm atomic.Bool
		config := env.pool.Config()
		config.MaxConns, config.MinConns, config.MinIdleConns = 1, 0, 0
		config.ShouldPing = func(context.Context, pgxpool.ShouldPingParams) bool { return false }
		dial := config.ConnConfig.DialFunc
		config.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := dial(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return &lostCompletionConn{Conn: conn, arm: &arm}, nil
		}
		writerPool, err := pgxpool.NewWithConfig(env.ctx, config)
		require.NoError(t, err, "create the writer's single-connection pool")
		t.Cleanup(writerPool.Close)
		require.NoError(t, writerPool.Ping(env.ctx), "open the writer connection before its result is lost")
		arm.Store(true)

		opts := env.options
		opts.Writer = db.NewLogService(writerPool, liveLogs)
		runIngest(t, env.ctx, opts)
		publishIngestEvent(t, env, "committed but its result was lost")
		nextCtx, cancel := context.WithTimeout(env.ctx, 15*time.Second)
		defer cancel()
		_, err = viewer.Next(nextCtx)
		require.ErrorIs(t, err, live.ErrLiveResync, "a possibly committed but unpublished row stops the live view instead of leaving a silent gap")
		waitIngestEmpty(t, env)
		require.False(t, arm.Load(), "the writer lost the result of a statement PostgreSQL had committed")
		var count int
		require.NoError(t, env.pool.QueryRow(env.ctx, "SELECT count(*) FROM logs").Scan(&count), "count logs through a separate connection")
		require.Equal(t, 1, count, "the retry recognizes the committed event instead of duplicating it")
	})

	t.Run("broker restart pauses ingest without losing deliveries", func(t *testing.T) {
		env := followBrokerRestarts(t, newIngestFixture(t))
		running := runIngest(t, env.ctx, env.options)
		stopped := func() bool {
			select {
			case <-running.done:
				return true
			default:
				return false
			}
		}
		publishIngestEvent(t, env, "before restart")
		waitIngestEmpty(t, env)
		require.Never(t, stopped, 500*time.Millisecond, 20*time.Millisecond, "an idle ingest keeps polling after committing a partial batch")

		stopTimeout := 10 * time.Second
		require.NoError(t, env.container.Stop(env.ctx, &stopTimeout), "stop the isolated broker while ingest waits for logs")
		require.Eventually(t, func() bool { return !env.conn.Conn.IsConnected() }, 15*time.Second, 20*time.Millisecond, "ingest observes the broker outage")
		require.Never(t, stopped, time.Second, 20*time.Millisecond, "a broker outage pauses ingest instead of stopping the server")
		require.NoError(t, env.container.Start(env.ctx), "restart the same broker with its persisted stream and durable")
		require.Eventually(t, env.conn.Conn.IsConnected, 30*time.Second, 50*time.Millisecond, "ingest reconnects to the restarted broker")

		payload, err := json.Marshal(rpc.LogLine{Timestamp: time.Now().UTC(), Message: "after restart", InstanceId: "web", Sequence: 43})
		require.NoError(t, err, "encode an event for the restarted broker")
		// A retried publish can store a second copy; event identity still yields one row.
		require.Eventually(t, func() bool {
			_, err := env.conn.JetStream.Publish(env.ctx, env.subject, payload)
			return err == nil
		}, 15*time.Second, 100*time.Millisecond, "the restarted broker accepts new logs")
		waitIngestEmpty(t, env)
		require.False(t, stopped(), "ingest keeps running after the broker returns")
		var count int
		require.NoError(t, env.pool.QueryRow(env.ctx, "SELECT count(*) FROM logs").Scan(&count), "count logs committed around the restart")
		require.Equal(t, 2, count, "events before and after the broker restart are each stored once")
	})

	t.Run("isolates a bad row but retains valid data on systemic SQL failure", func(t *testing.T) {
		env := newIngestFixture(t)
		opts := env.options
		opts.Writer = ingestWriterFunc(func(ctx context.Context, input []db.IngestLog) error {
			for _, event := range input {
				switch event.Log.LogLine {
				case "bad":
					return &pgconn.PgError{Code: "54000", Message: "index entry too large"}
				case "schema":
					return &pgconn.PgError{Code: "42P01", Message: "relation missing"}
				}
			}
			return env.writer.CommitLogs(ctx, input)
		})
		running := runIngest(t, env.ctx, opts)
		publishIngestEvent(t, env, "left")
		publishIngestEvent(t, env, "bad")
		publishIngestEvent(t, env, "right")
		waitIngestEmpty(t, env)
		var count int
		require.NoError(t, env.pool.QueryRow(env.ctx, "SELECT count(*) FROM logs").Scan(&count), "count rows after isolating a bad event")
		require.Equal(t, 2, count, "a permanent row failure must not drop the other rows in the batch")
		publishIngestEvent(t, env, "schema")
		select {
		case <-running.done:
		case <-env.ctx.Done():
			t.Fatal("systemic SQL errors must stop the ingest rather than terminate valid logs")
		}
		require.ErrorContains(t, running.err, "systemic ingest write failure", "a schema error is not a poison log")
		info, err := env.conn.JetStream.Stream(env.ctx, "LOGS")
		require.NoError(t, err, "inspect durable backlog after a systemic failure")
		require.Equal(t, uint64(1), info.CachedInfo().State.Msgs, "the valid event remains in the broker without ACK or Term")
		runIngest(t, env.ctx, env.options)
		waitIngestEmpty(t, env)
		require.NoError(t, env.pool.QueryRow(env.ctx, "SELECT count(*) FROM logs").Scan(&count), "read logs after recovery")
		require.Equal(t, 3, count, "restarting after fixing the schema stores the retained valid event")
	})

	t.Run("shutdown deadline cancels writes without acknowledging their messages", func(t *testing.T) {
		env := newIngestFixture(t)
		entered := make(chan struct{})
		opts := env.options
		opts.Writer = ingestWriterFunc(func(ctx context.Context, input []db.IngestLog) error {
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		})
		running := runIngest(t, env.ctx, opts)
		publishIngestEvent(t, env, "interrupted batch")
		select {
		case <-entered:
		case <-env.ctx.Done():
			t.Fatal("writer must begin before its shutdown deadline is tested")
		}
		shutdownCtx, cancel := context.WithTimeout(env.ctx, 50*time.Millisecond)
		defer cancel()
		require.ErrorIs(t, running.service.Shutdown(shutdownCtx), context.DeadlineExceeded, "deadline exhaustion interrupts rather than falsely completes the write")
		<-running.done
		require.ErrorIs(t, running.err, context.Canceled, "shutdown joins the canceled writer before returning")
		runIngest(t, env.ctx, env.options)
		waitIngestEmpty(t, env)
		var count int
		require.NoError(t, env.pool.QueryRow(env.ctx, "SELECT count(*) FROM logs").Scan(&count), "count recovered interrupted messages")
		require.Equal(t, 1, count, "a new process stores the unacknowledged event after shutdown")
	})
}

func TestIngestBrokerBootstrap(t *testing.T) {
	t.Run("legacy migration is gated preserves cursor and refuses a missing durable", func(t *testing.T) {
		env := newIngestFixture(t)
		js := env.conn.JetStream
		_, err := js.CreateStream(env.ctx, jetstream.StreamConfig{Name: "LOGS", Subjects: []string{"logs.>"}, Storage: jetstream.FileStorage, MaxBytes: 1 << 30, MaxAge: 7 * 24 * time.Hour})
		require.NoError(t, err, "create the legacy stream without a consumer")
		opts := env.options
		opts.AllowRetentionMigration = true
		_, err = ingest.Open(env.ctx, opts)
		require.ErrorContains(t, err, "durable is missing", "legacy migration cannot guess a lost consumer cursor")
		consumer, err := js.CreateConsumer(env.ctx, "LOGS", jetstream.ConsumerConfig{Durable: "log-processor", FilterSubject: "logs.>", AckPolicy: jetstream.AckExplicitPolicy})
		require.NoError(t, err, "create the existing legacy durable")
		_, err = js.CreateConsumer(env.ctx, "LOGS", jetstream.ConsumerConfig{Durable: "unexpected", FilterSubject: "logs.>", AckPolicy: jetstream.AckExplicitPolicy})
		require.NoError(t, err, "simulate an additional legacy consumer on the isolated broker")
		_, err = ingest.Open(env.ctx, opts)
		require.ErrorContains(t, err, "unexpected LOGS consumer", "migration must not silently adopt another consumer's interest")
		require.NoError(t, js.DeleteConsumer(env.ctx, "LOGS", "unexpected"), "remove only the extra test consumer before the valid migration")
		publishIngestEvent(t, env, "old acknowledged")
		msg, err := consumer.Next(jetstream.FetchMaxWait(time.Second))
		require.NoError(t, err, "fetch one old message")
		require.NoError(t, msg.DoubleAck(env.ctx), "persist the legacy durable cursor before migration")
		publishIngestEvent(t, env, "pending migration")
		before, err := consumer.Info(env.ctx)
		require.NoError(t, err, "remember legacy consumer state")
		_, err = ingest.Open(env.ctx, env.options)
		require.ErrorContains(t, err, "migration disabled", "legacy retention is never changed without explicit consent")
		unchanged, err := js.Stream(env.ctx, "LOGS")
		require.NoError(t, err, "read the gated legacy stream")
		require.Equal(t, jetstream.LimitsPolicy, unchanged.CachedInfo().Config.Retention, "a refused migration must not change retention")
		var wg sync.WaitGroup
		errors := make([]error, 2)
		for i := range 2 {
			wg.Go(func() { _, errors[i] = ingest.Open(env.ctx, opts) })
		}
		wg.Wait()
		for _, err := range errors {
			require.NoError(t, err, "concurrent replica startup shares the PostgreSQL bootstrap lock")
		}
		consumer, err = js.Consumer(env.ctx, "LOGS", "log-processor")
		require.NoError(t, err, "reload the same durable after migration")
		after, err := consumer.Info(env.ctx)
		require.NoError(t, err, "inspect the preserved cursor")
		require.Equal(t, before.AckFloor.Stream, after.AckFloor.Stream, "migration must not reset the acknowledged stream position")
		require.Equal(t, before.Delivered.Stream, after.Delivered.Stream, "migration must not reset the delivered stream position")
		stream, err := js.Stream(env.ctx, "LOGS")
		require.NoError(t, err, "read migrated stream settings")
		require.Equal(t, jetstream.InterestPolicy, stream.CachedInfo().Config.Retention, "legacy migration uses the compatible InterestPolicy conversion")
		require.Zero(t, stream.CachedInfo().Config.MaxAge, "pending messages cannot expire due to age")
		require.NoError(t, js.DeleteConsumer(env.ctx, "LOGS", "log-processor"), "simulate an operator deleting the durable on the isolated test broker")
		_, err = ingest.Open(env.ctx, opts)
		require.ErrorContains(t, err, "durable is missing", "a prepared Interest stream without its durable is an incident, not a bootstrap")
		_, err = js.Consumer(env.ctx, "LOGS", "log-processor")
		require.ErrorIs(t, err, jetstream.ErrConsumerNotFound, "failed startup must not recreate the missing durable")
	})

	t.Run("full broker retains accepted messages across app restart and rejects new ones", func(t *testing.T) {
		env := newIngestFixture(t)
		opts := env.options
		opts.StreamMaxBytes = 1200
		service, err := ingest.Open(env.ctx, opts)
		require.NoError(t, err, "bootstrap a small isolated broker without starting processing")
		publishIngestEvent(t, env, strings.Repeat("a", 700))
		payload, err := json.Marshal(rpc.LogLine{Timestamp: time.Now().UTC(), Message: strings.Repeat("b", 700)})
		require.NoError(t, err, "encode the next event")
		_, err = env.conn.JetStream.Publish(env.ctx, env.subject, payload)
		require.Error(t, err, "a full stream refuses new data instead of evicting an accepted event")
		require.NoError(t, service.Shutdown(env.ctx), "stopping an idle app must not delete its durable")
		restarted, err := ingest.Open(env.ctx, env.options)
		require.NoError(t, err, "reopening preserves the operator's finite broker limit")
		stream, err := env.conn.JetStream.Stream(env.ctx, "LOGS")
		require.NoError(t, err, "inspect retained backlog before restarting processing")
		require.Equal(t, uint64(1), stream.CachedInfo().State.Msgs, "the first accepted event survives an application restart")
		require.Equal(t, int64(1200), stream.CachedInfo().Config.MaxBytes, "default options must not replace an existing capacity")
		runOpenedIngest(t, env.ctx, restarted)
		waitIngestEmpty(t, env)
		_, err = env.conn.JetStream.Publish(env.ctx, env.subject, payload)
		require.NoError(t, err, "committing and acknowledging the first event frees capacity for the rejected event")
		waitIngestEmpty(t, env)
		var count int
		require.NoError(t, env.pool.QueryRow(env.ctx, "SELECT count(*) FROM logs").Scan(&count), "count both committed events")
		require.Equal(t, 2, count, "neither accepted event is evicted when broker storage fills")
	})
}

type ingestWriterFunc func(context.Context, []db.IngestLog) error

func (f ingestWriterFunc) CommitLogs(ctx context.Context, input []db.IngestLog) error {
	return f(ctx, input)
}

type ingestFixture struct {
	ctx       context.Context
	pool      *pgxpool.Pool
	container *natscontainer.NATSContainer
	conn      *natsconn.NatsConnection
	writer    *db.PostgresLogService
	options   ingest.Options
	subject   string
	scope     live.Scope
}

func newIngestFixture(t *testing.T) ingestFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	t.Cleanup(cancel)
	pool := testutils.NewPostgresPool(t, ctx)
	project, err := projects.NewPostgresProjectsService(pool).CreateProject(ctx, "Ingest", []string{"api"})
	require.NoError(t, err, "create a client for the durable-ingest contract")
	container, err := natscontainer.Run(ctx, "nats:2.15.1-alpine", natscontainer.WithConfigFile(strings.NewReader("jetstream { store_dir: /data }")))
	if container != nil {
		testcontainers.CleanupContainer(t, container)
	}
	require.NoError(t, err, "start an isolated JetStream broker")
	url, err := container.ConnectionString(ctx)
	require.NoError(t, err, "resolve the isolated broker address")
	conn, err := natsconn.NewNatsConnection(natsconn.NatsConnectionConfig{NatsAddr: url, EnableJetStream: true})
	require.NoError(t, err, "connect without implicitly changing stream configuration")
	t.Cleanup(conn.Close)
	writer := db.NewLogService(pool, nil)
	return ingestFixture{
		ctx: ctx, pool: pool, container: container, conn: conn, writer: writer,
		options: ingest.Options{Connection: conn, Pool: pool, Writer: writer},
		subject: "logs." + project.ID.String() + "." + project.Clients[0].ID.String(),
		scope:   live.Scope{ProjectID: project.ID, ClientID: project.Clients[0].ID},
	}
}

// followBrokerRestarts replaces the fixture connection with one that redials
// the fixture's own container, whose host port can change after a restart.
func followBrokerRestarts(t *testing.T, env ingestFixture) ingestFixture {
	t.Helper()
	url, err := env.container.ConnectionString(env.ctx)
	require.NoError(t, err, "resolve the isolated broker address")
	nc, err := nats.Connect(url, nats.SetCustomDialer(containerDialer{ctx: env.ctx, container: env.container}),
		nats.MaxReconnects(-1), nats.ReconnectWait(100*time.Millisecond))
	require.NoError(t, err, "connect through the restart-following dialer")
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	require.NoError(t, err, "open JetStream on the restart-following connection")
	env.conn = &natsconn.NatsConnection{Conn: nc, JetStream: js}
	env.options.Connection = env.conn
	return env
}

type containerDialer struct {
	ctx       context.Context
	container *natscontainer.NATSContainer
}

func (d containerDialer) Dial(network, _ string) (net.Conn, error) {
	address, err := d.container.PortEndpoint(d.ctx, "4222/tcp", "")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(d.ctx, 2*time.Second)
	defer cancel()
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, address)
}

// lostCompletionConn fails a PostgreSQL connection after a statement has
// committed but before pgx reads its result. Once armed, it drops the
// ReadyForQuery that follows the next CommandComplete; PostgreSQL sends it
// only after the implicit transaction commits.
type lostCompletionConn struct {
	net.Conn
	arm      *atomic.Bool
	pending  []byte
	ready    []byte
	complete bool
	lost     bool
}

// pgx serializes reads, so the framing state needs no lock.
func (c *lostCompletionConn) Read(p []byte) (int, error) {
	for len(c.ready) == 0 {
		if c.lost {
			return 0, io.EOF
		}
		buf := make([]byte, 32<<10)
		n, err := c.Conn.Read(buf)
		c.pending = append(c.pending, buf[:n]...)
		for len(c.pending) >= 5 && !c.lost {
			size := 1 + int(binary.BigEndian.Uint32(c.pending[1:5]))
			if len(c.pending) < size {
				break
			}
			frame := c.pending[:size]
			c.pending = c.pending[size:]
			switch {
			case frame[0] == 'C' && c.arm.Load():
				c.complete = true
			case frame[0] == 'Z' && c.complete && c.arm.CompareAndSwap(true, false):
				c.lost = true
				_ = c.Conn.Close()
				continue
			}
			c.ready = append(c.ready, frame...)
		}
		if err != nil && len(c.ready) == 0 && !c.lost {
			return 0, err
		}
	}
	n := copy(p, c.ready)
	c.ready = c.ready[n:]
	return n, nil
}

type runningIngest struct {
	service *ingest.Service
	done    chan struct{}
	err     error
}

func runIngest(t *testing.T, ctx context.Context, opts ingest.Options) *runningIngest {
	t.Helper()
	service, err := ingest.Open(ctx, opts)
	require.NoError(t, err, "prepare durable ingest before starting the worker")
	return runOpenedIngest(t, ctx, service)
}

func runOpenedIngest(t *testing.T, ctx context.Context, service *ingest.Service) *runningIngest {
	t.Helper()
	runCtx, cancel := context.WithCancel(ctx)
	running := &runningIngest{service: service, done: make(chan struct{})}
	go func() { running.err = service.Run(runCtx); close(running.done) }()
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = service.Shutdown(shutdownCtx)
		cancel()
		select {
		case <-running.done:
		case <-shutdownCtx.Done():
			t.Error("ingest must exit before closing borrowed connections")
		}
	})
	return running
}

func publishIngestEvent(t *testing.T, env ingestFixture, message string) []byte {
	t.Helper()
	payload, err := json.Marshal(rpc.LogLine{Timestamp: time.Now().UTC(), Message: message, InstanceId: "web", Sequence: 42})
	require.NoError(t, err, "encode a valid original event")
	_, err = env.conn.JetStream.Publish(env.ctx, env.subject, payload)
	require.NoError(t, err, "broker accepts the event for durable processing")
	return payload
}

func waitIngestEmpty(t *testing.T, env ingestFixture) {
	t.Helper()
	require.Eventually(t, func() bool {
		stream, err := env.conn.JetStream.Stream(env.ctx, "LOGS")
		return err == nil && stream.CachedInfo().State.Msgs == 0
	}, 15*time.Second, 40*time.Millisecond, "every accepted event is committed or explicitly rejected before broker removal")
}
