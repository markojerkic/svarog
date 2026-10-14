package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/ingest"
	websocket "github.com/markojerkic/svarog/internal/server/web-socket"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	natscontainer "github.com/testcontainers/testcontainers-go/modules/nats"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestPostgresClientsIngestIntoPostgresLogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Set up PostgreSQL projects with distinct client UUIDs, JetStream, and Mongo log storage.
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool)
	first, err := projectService.CreateProject(ctx, "First", []string{"api"})
	require.NoError(t, err, "create first PG project with active client")
	second, err := projectService.CreateProject(ctx, "Second", []string{"api"})
	require.NoError(t, err, "create second PG project with same client name and a distinct UUID")

	natsContainer, err := natscontainer.Run(ctx, "nats:2.11-alpine",
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
		JetStreamConfig: natsconn.JetStreamConfig{
			Name:     "LOGS",
			Subjects: []string{"logs.>"},
		},
	})
	require.NoError(t, err, "connect to JetStream and create log stream")
	t.Cleanup(conn.Close)

	mongoContainer, err := mongodb.Run(ctx, "mongo:7")
	if mongoContainer != nil {
		testcontainers.CleanupContainer(t, mongoContainer)
	}
	require.NoError(t, err, "start MongoDB for unmigrated log reads")
	mongoURL, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err, "get MongoDB connection string")
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
	require.NoError(t, err, "connect MongoDB log storage")
	t.Cleanup(func() {
		require.NoError(t, mongoClient.Disconnect(context.Background()), "close MongoDB connection")
	})

	// Run ingestion and log aggregation together; wait for both before cleaning up storage.
	logService := db.NewLogService(mongoClient.Database("svarog_test"), pool,
		websocket.NewWsLogLineRenderer(websocket.NewWatchHub(conn.Conn)))
	logServer := db.NewLogServer(logService)
	channel := make(chan db.LogLineWithHost, 100)
	ingestService := ingest.NewIngestService(channel, conn, pool)
	runCtx, stop := context.WithCancel(ctx)
	ingestDone := make(chan error, 1)
	logsDone := make(chan struct{})
	go func() { ingestDone <- ingestService.Run(runCtx) }()
	go func() {
		logServer.Run(runCtx, channel)
		close(logsDone)
	}()
	t.Cleanup(func() {
		stop()
		ingestService.Stop()
		select {
		case err := <-ingestDone:
			require.NoError(t, err, "ingest loop stops without error")
		case <-time.After(5 * time.Second):
			t.Error("ingest loop did not stop")
		}
		select {
		case <-logsDone:
		case <-time.After(5 * time.Second):
			t.Error("log aggregation did not stop")
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

	// Valid project/client UUID pairs are stored in PostgreSQL. Reads remain on MongoDB.
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
	publish("mongo-object-id", "client-name", "legacy ids rejected")
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
