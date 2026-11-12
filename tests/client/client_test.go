package client_test

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/cmd/client/reader"
	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/lib/svarogclient"
	"github.com/markojerkic/svarog/internal/lib/svaroglog"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	natscontainer "github.com/testcontainers/testcontainers-go/modules/nats"
)

func TestSharedClientDelivery(t *testing.T) {
	broker := newClientBroker(t)

	t.Run("both adapters preserve records and share sequence ordering", func(t *testing.T) {
		connectionURL, stream, subject := broker.stream(t, jetstream.StreamConfig{})
		client := newClient(t, connectionURL)
		installLogger(t, client)

		require.NoError(t, reader.NewReader(strings.NewReader("\x1b[31mstdin line\x1b[0m\ninvalid\x00line\nafter invalid\nbad \xff\xfe bytes\n"), client.Enqueue).Run(broker.ctx),
			"the stdin adapter must submit valid messages, repair invalid UTF-8 and continue after locally rejected source data")
		at := time.Date(2026, 11, 12, 10, 0, 0, 123456789, time.UTC)
		record := slog.NewRecord(at, slog.LevelWarn, "request failed", 0)
		record.AddAttrs(slog.Int("status", 500))
		handler := svaroglog.NewHandler(client, svaroglog.Options{}).
			WithAttrs([]slog.Attr{slog.String("service", "api")}).WithGroup("request")
		require.NoError(t, handler.Handle(context.Background(), record), "the slog adapter must accept a structured record")
		log.Printf("standard log: port=%d", 1323)
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		slog.InfoContext(canceled, "request context canceled", "reason", "test")

		var producers sync.WaitGroup
		for i := range 8 {
			producers.Go(func() {
				slog.With("producer", i).WithGroup("request").Info("concurrent entry", "id", i)
			})
		}
		producers.Wait()
		shutdownClient(t, client)

		info, err := stream.Info(broker.ctx)
		require.NoError(t, err, "read the stream after all accepted entries were acknowledged")
		require.EqualValues(t, 14, info.State.Msgs, "both sources and concurrent child loggers must persist every accepted entry")
		for i := range 14 {
			message, err := stream.GetMsg(broker.ctx, uint64(i+1))
			require.NoError(t, err, "every accepted entry must be readable after shutdown")
			var line rpc.LogLine
			require.NoError(t, json.Unmarshal(message.Data, &line), "messages must retain the existing RPC format")
			require.NoError(t, logevent.Validate(line), "every emitted payload must satisfy ingest validation")
			require.Equal(t, i, line.Sequence, "both adapters must share one ordered, collision-free sequence")
			require.Equal(t, "integration", line.InstanceId, "the client, not the source adapter, must own instance assignment")
			key := logevent.MessageKey(subject, message.Data)
			require.Equal(t, hex.EncodeToString(key[:]), message.Header.Get(jetstream.MsgIDHeader), "broker identity must derive from the exact original subject and payload")
			switch i {
			case 0:
				require.Equal(t, "stdin line", line.Message, "stdin ANSI styling must not reach stored messages")
			case 1:
				require.Equal(t, "after invalid", line.Message, "a malformed source line must not stop delivery of later valid messages")
			case 2:
				require.Equal(t, "bad \uFFFD\uFFFD bytes", line.Message, "invalid UTF-8 must keep the whole line, with one replacement character per invalid byte")
			case 3:
				require.True(t, line.Timestamp.Equal(at), "slog timestamps must retain the original nanoseconds rather than the worker's publish time")
				require.Contains(t, line.Message, "level=WARN", "formatted messages must retain severity")
				require.Contains(t, line.Message, "service=api request.status=500", "attributes before and after WithGroup must retain their namespaces")
				require.NotContains(t, line.Message, "time=", "the transport timestamp must not be duplicated in the formatted message")
			case 4:
				require.Contains(t, line.Message, "standard log: port=1323", "global standard log calls must enter the same sender")
			case 5:
				require.Contains(t, line.Message, "request context canceled", "request cancellation must not cancel log delivery")
			default:
				require.Contains(t, line.Message, "request.id=", "concurrent derived handlers must retain grouped attributes")
			}
		}
	})

	t.Run("a lost acknowledgement retries unchanged data and backpressures producers", func(t *testing.T) {
		connectionURL, stream, subject := broker.stream(t, jetstream.StreamConfig{NoAck: true})
		subscription, err := broker.nc.SubscribeSync(subject)
		require.NoError(t, err, "observe publish attempts without replacing the real JetStream broker")
		defer subscription.Unsubscribe()
		require.NoError(t, broker.nc.Flush(), "install observation before the first publish")
		client := newClient(t, connectionURL)
		// Retry diagnostics must stay local even while global logging uses this client.
		installLogger(t, client)
		require.NoError(t, client.Enqueue(svarogclient.Entry{Message: "first"}), "accept the original message locally")
		first, err := subscription.NextMsg(5 * time.Second)
		require.NoError(t, err, "the first attempt must reach the broker despite its lost acknowledgement")
		require.Eventually(t, func() bool {
			info, err := stream.Info(broker.ctx)
			return err == nil && info.State.Msgs == 1
		}, 3*time.Second, 10*time.Millisecond, "the broker must store the first attempt before acknowledgements are enabled")

		for i := range 16 {
			require.NoError(t, client.Enqueue(svarogclient.Entry{Message: fmt.Sprintf("queued %d", i)}), "the fixed-size queue must accept its remaining capacity")
		}
		blocked := make(chan error, 1)
		go func() { blocked <- client.Enqueue(svarogclient.Entry{Message: "waited for space"}) }()
		select {
		case err := <-blocked:
			t.Fatalf("a producer must block instead of dropping or bypassing a full queue; returned %v", err)
		case <-time.After(100 * time.Millisecond):
		}

		info, err := stream.Info(broker.ctx)
		require.NoError(t, err, "read the stream before restoring acknowledgements")
		config := info.Config
		config.NoAck = false
		_, err = broker.js.UpdateStream(broker.ctx, config)
		require.NoError(t, err, "restore acknowledgements without deleting the already stored message")
		retried, err := subscription.NextMsg(8 * time.Second)
		require.NoError(t, err, "an uncertain publish must be retried rather than discarded")
		require.Equal(t, first.Data, retried.Data, "retry must preserve timestamp, sequence, truncation and all encoded bytes")
		require.Equal(t, first.Header.Get(jetstream.MsgIDHeader), retried.Header.Get(jetstream.MsgIDHeader), "a lost acknowledgement must not create a new event identity")
		select {
		case err := <-blocked:
			require.NoError(t, err, "the waiting producer must be admitted after publishing resumes")
		case <-time.After(5 * time.Second):
			t.Fatal("publishing recovery must release the producer blocked by backpressure")
		}
		shutdownClient(t, client)
		info, err = stream.Info(broker.ctx)
		require.NoError(t, err, "read final retained messages after a successful drain")
		require.EqualValues(t, 18, info.State.Msgs, "all accepted events must persist exactly once within broker deduplication, without retry diagnostics entering the stream")
	})

	t.Run("permanent rejection cannot masquerade as a successful drain", func(t *testing.T) {
		connectionURL, _, _ := broker.stream(t, jetstream.StreamConfig{MaxMsgSize: 64})
		client := newClient(t, connectionURL)
		require.NoError(t, client.Enqueue(svarogclient.Entry{Message: strings.Repeat("x", 128)}), "local admission does not claim broker persistence")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := client.Shutdown(ctx)
		require.ErrorContains(t, err, "broker rejected the payload size", "a permanent server rejection must stop the sender and surface through shutdown")
		require.NotErrorIs(t, err, context.DeadlineExceeded, "an unrecoverable payload must not retry until the drain deadline")
	})
}

func TestClientShutdownInterruptsHandshakeAndBlockedAdmission(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "listen as a peer that accepts TCP but never completes the NATS handshake")
	t.Cleanup(func() { _ = listener.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	connectionURL := fmt.Sprintf("svarog://%s/logs.%s.%s?token=%s", listener.Addr(), uuid.NewString(), uuid.NewString(), url.QueryEscape(clientToken(t)))
	client := newClient(t, connectionURL)
	require.NoError(t, client.Enqueue(svarogclient.Entry{Message: "in flight"}), "accept an entry before the peer stalls its handshake")
	var peer net.Conn
	select {
	case peer = <-accepted:
		t.Cleanup(func() { _ = peer.Close() })
	case <-time.After(5 * time.Second):
		t.Fatal("the background sender must attempt to connect for its accepted entry")
	}
	for range 16 {
		require.NoError(t, client.Enqueue(svarogclient.Entry{Message: "queued"}), "buffer accepted entries while the initial handshake is blocked")
	}
	blocked := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		blocked <- client.Enqueue(svarogclient.Entry{Message: "not accepted"})
	}()
	<-started
	select {
	case err := <-blocked:
		t.Fatalf("a producer must wait while the handshake stalls and the queue is full; returned %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- client.Shutdown(ctx) }()
	select {
	case err := <-shutdownDone:
		require.ErrorIs(t, err, context.DeadlineExceeded, "expiry must report that accepted entries were not confirmed")
	case <-time.After(time.Second):
		t.Fatal("shutdown must interrupt a stalled handshake and join the publisher instead of waiting for its network timeout")
	}
	select {
	case err := <-blocked:
		require.ErrorIs(t, err, svarogclient.ErrClosed, "shutdown must release a producer waiting for queue space")
	case <-time.After(time.Second):
		t.Fatal("a blocked producer must not outlive shutdown")
	}
	require.NoError(t, peer.SetReadDeadline(time.Now().Add(time.Second)), "bound observation of the peer socket closing")
	var data [1]byte
	_, err = peer.Read(data[:])
	require.ErrorIs(t, err, io.EOF, "shutdown must close the stalled socket before returning")
	require.ErrorIs(t, client.Shutdown(context.Background()), context.DeadlineExceeded, "repeated shutdown must retain the original failed drain result")
}

type clientBroker struct {
	ctx   context.Context
	nc    *nats.Conn
	js    jetstream.JetStream
	host  string
	token string
}

func newClientBroker(t *testing.T) clientBroker {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := natscontainer.Run(ctx, "nats:2.15.1-alpine", natscontainer.WithConfigFile(strings.NewReader("jetstream { store_dir: /data }")))
	if container != nil {
		testcontainers.CleanupContainer(t, container)
	}
	require.NoError(t, err, "start an isolated broker for client delivery contracts")
	address, err := container.ConnectionString(ctx)
	require.NoError(t, err, "resolve the isolated broker address")
	nc, err := nats.Connect(address)
	require.NoError(t, err, "open an administrative connection to inspect client delivery")
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	require.NoError(t, err, "open JetStream without changing application-owned broker configuration")
	return clientBroker{ctx: ctx, nc: nc, js: js, host: strings.TrimPrefix(address, "nats://"), token: clientToken(t)}
}

func (b clientBroker) stream(t *testing.T, config jetstream.StreamConfig) (string, jetstream.Stream, string) {
	t.Helper()
	subject := "logs." + uuid.NewString() + "." + uuid.NewString()
	config.Name = "CLIENT_" + uuid.NewString()
	config.Subjects = []string{subject}
	config.Storage = jetstream.MemoryStorage
	stream, err := b.js.CreateStream(b.ctx, config)
	require.NoError(t, err, "create a test-owned stream for an isolated client subject")
	connectionURL := fmt.Sprintf("svarog://%s/%s?token=%s", b.host, subject, url.QueryEscape(b.token))
	return connectionURL, stream, subject
}

func newClient(t *testing.T, connectionURL string) *svarogclient.Client {
	t.Helper()
	client, err := svarogclient.New(svarogclient.Options{ConnectionString: connectionURL, InstanceID: "integration", Diagnostics: io.Discard})
	require.NoError(t, err, "valid local configuration must create a client without depending on broker availability")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = client.Shutdown(ctx)
	})
	return client
}

func shutdownClient(t *testing.T, client *svarogclient.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, client.Shutdown(ctx), "successful shutdown must wait for broker acknowledgements for every accepted entry")
}

func installLogger(t *testing.T, client *svarogclient.Client) {
	t.Helper()
	previous := slog.Default()
	output, flags, prefix := log.Writer(), log.Flags(), log.Prefix()
	level := slog.SetLogLoggerLevel(slog.LevelInfo)
	t.Cleanup(func() {
		slog.SetDefault(previous)
		slog.SetLogLoggerLevel(level)
		log.SetOutput(output)
		log.SetFlags(flags)
		log.SetPrefix(prefix)
	})
	svaroglog.Setup(client, svaroglog.Options{Level: slog.LevelDebug})
}

func clientToken(t *testing.T) string {
	t.Helper()
	account, err := nkeys.CreateAccount()
	require.NoError(t, err, "create a signer for local credential fixtures")
	defer account.Wipe()
	user, err := nkeys.CreateUser()
	require.NoError(t, err, "create a user key for the client credentials")
	defer user.Wipe()
	public, err := user.PublicKey()
	require.NoError(t, err, "read the credential fixture's public user key")
	token, err := jwt.NewUserClaims(public).Encode(account)
	require.NoError(t, err, "sign a syntactically valid user JWT")
	seed, err := user.Seed()
	require.NoError(t, err, "read the credential fixture's seed")
	credentials, err := jwt.FormatUserConfig(token, seed)
	require.NoError(t, err, "format credentials exactly as generated connection URLs do")
	return base64.StdEncoding.EncodeToString(credentials)
}
