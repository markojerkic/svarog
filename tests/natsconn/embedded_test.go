package natsconn_test

import (
	"context"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/lib/natsconn/embedded"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/lib/svarogclient"
	"github.com/markojerkic/svarog/internal/server/ingest"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedBrokerIngestSurvivesRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	t.Cleanup(cancel)
	pool := testutils.NewPostgresPool(t, ctx)
	projectService := projects.NewPostgresProjectsService(pool, nil)
	project, err := projectService.Create(ctx, projects.ProjectInput{Name: "Embedded ingest", Clients: []string{"api"}})
	require.NoError(t, err, "create the project and client that own the ingested logs")
	clientID := project.Clients[0].ID

	account, err := nkeys.CreateAccount()
	require.NoError(t, err, "create a stable APP account for both broker runs")
	defer account.Wipe()
	seed, err := account.Seed()
	require.NoError(t, err, "obtain the APP seed for the broker and credential service")
	cfg := embedded.Config{Host: "127.0.0.1", StoreDir: t.TempDir(), AccountSeed: string(seed)}

	var connectionURL string
	publishAndCommit := func(t *testing.T, conn *natsconn.NatsConnection, message string, sequence uint64) {
		t.Helper()
		service, err := ingest.Open(ctx, ingest.Options{
			Connection: conn, Pool: pool, Writer: logstore.New(pool),
		})
		require.NoError(t, err, "prepare the LOGS stream and durable through ingest")
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- service.Run(runCtx) }()
		t.Cleanup(func() {
			shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			err := service.Shutdown(shutdownCtx)
			cancel()
			require.NoError(t, err, "drain ingest before closing its broker connection")
			select {
			case err := <-done:
				require.NoError(t, err, "ingest exits cleanly after draining")
			case <-shutdownCtx.Done():
				t.Error("ingest must exit before its NATS connection closes")
			}
		})

		client, err := svarogclient.New(svarogclient.Options{ConnectionString: connectionURL, InstanceID: "embedded-test"})
		require.NoError(t, err, "open the issued connection URL using the production publisher")
		publishCtx, stop := context.WithTimeout(ctx, 10*time.Second)
		defer stop()
		defer func() { _ = client.Shutdown(publishCtx) }()
		require.NoError(t, client.Enqueue(svarogclient.Entry{Message: message}), "enqueue the log for publication over TCP")
		require.NoError(t, client.Shutdown(publishCtx), "wait for the broker to accept the authenticated publish")

		consumer, err := conn.JetStream.Consumer(ctx, "LOGS", "log-processor")
		require.NoError(t, err, "find the durable that processes the published log")
		require.Eventually(t, func() bool {
			info, err := consumer.Info(ctx)
			return err == nil && info.AckFloor.Stream == sequence
		}, 10*time.Second, 50*time.Millisecond, "ingest acknowledges the log after committing it")
		var count int
		err = pool.QueryRow(ctx, "SELECT count(*) FROM logs WHERE client_id = $1 AND instance_name = $2 AND message = $3",
			clientID, "embedded-test", message).Scan(&count)
		require.NoError(t, err, "read the committed log from PostgreSQL")
		require.Equal(t, 1, count, "PostgreSQL contains exactly one row for the published log")
	}

	if !t.Run("initial broker", func(t *testing.T) {
		conn := openEmbedded(t, ctx, cfg)
		addr, err := url.Parse(conn.Conn.ConnectedUrl())
		require.NoError(t, err, "read the broker's actual TCP listener address")
		cfg.Port, err = strconv.Atoi(addr.Port())
		require.NoError(t, err, "reuse the OS-assigned port when restarting the broker")

		credentials, err := serverauth.NewNatsCredentialService(cfg.AccountSeed, addr.Host, projectService)
		require.NoError(t, err, "create the credential service with the broker's APP account")
		clientConfig, err := credentials.GenerateConnString(ctx, serverauth.CredentialGenerationRequest{
			ProjectID: project.ID.String(), ClientID: clientID.String(),
		})
		require.NoError(t, err, "issue the client's authenticated connection URL")
		connectionURL = clientConfig.GetConnString()
		publishAndCommit(t, conn, "before broker restart", 1)
	}) {
		return
	}

	t.Run("restarted broker", func(t *testing.T) {
		conn := openEmbedded(t, ctx, cfg)
		stream, err := conn.JetStream.Stream(ctx, "LOGS")
		require.NoError(t, err, "recover the persisted stream before opening ingest again")
		require.Equal(t, uint64(1), stream.CachedInfo().State.LastSeq, "the recovered stream retains its original sequence")
		consumer, err := conn.JetStream.Consumer(ctx, "LOGS", "log-processor")
		require.NoError(t, err, "recover the persisted durable before opening ingest again")
		info, err := consumer.Info(ctx)
		require.NoError(t, err, "read the recovered durable's cursor")
		require.Equal(t, uint64(1), info.AckFloor.Stream, "the recovered durable retains its acknowledged position")

		publishAndCommit(t, conn, "after broker restart", 2)
	})
}

func openEmbedded(t *testing.T, ctx context.Context, cfg embedded.Config) *natsconn.NatsConnection {
	t.Helper()
	broker := embedded.New(cfg)
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, broker.Shutdown(shutdownCtx), "shut down the broker before reusing its JetStream directory")
	})
	require.NoError(t, broker.Start(ctx), "start embedded NATS and wait for the TCP listener")
	conn, err := broker.Connect(natsconn.ConnectOptions{Name: "svarog-ingest", EnableJetStream: true})
	require.NoError(t, err, "create the in-process connection with JetStream access")
	t.Cleanup(conn.Close)
	return conn
}
