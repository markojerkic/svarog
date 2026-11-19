package client_test

import (
	"bufio"
	"bytes"
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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	natscontainer "github.com/testcontainers/testcontainers-go/modules/nats"

	"github.com/markojerkic/svarog/cmd/client/reader"
	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/lib/svarogclient"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/svaroglog"
)

func TestPackagedJavaBridge(t *testing.T) {
	classpath := os.Getenv("SVAROG_JAVA_SMOKE_CLASSPATH")
	if classpath == "" {
		t.Skip("requires the packaged Logback JAR and its dependencies")
	}
	java := "java"
	if home := os.Getenv("JAVA_HOME"); home != "" {
		java = filepath.Join(home, "bin", "java")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, java, "-cp", classpath, "java/smoke/PackagedBridge.java")
	command.Dir = filepath.Join("..", "..")
	// Readiness only validates local configuration. Without any log records this
	// test does not need a broker or send the generated credentials over the network.
	command.Env = append(os.Environ(),
		"SVAROG_BRIDGE_PATH=",
		"SVAROG_INSTANCE_ID=packaged-java-smoke",
		"SVAROG_CONN_STRING=svarog://127.0.0.1:1/logs.smoke?token="+url.QueryEscape(clientToken(t)),
	)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "smoke-test the bundled bridge outside the Maven test classpath: %s", output)
}

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
		handler := svarogclient.NewHandler(client, svarogclient.HandlerOptions{}).
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

	t.Run("bridge preserves source records and drains on EOF or failure", func(t *testing.T) {
		binary := filepath.Join(t.TempDir(), "svarog-bridge")
		build := exec.CommandContext(broker.ctx, "go", "build", "-o", binary, "./cmd/svarog-bridge")
		build.Dir = filepath.Join("..", "..")
		output, err := build.CombinedOutput()
		require.NoError(t, err, "build the real bridge executable: %s", output)

		for _, connectionURL := range []string{"", "not-a-svarog-url"} {
			command := exec.CommandContext(broker.ctx, binary)
			command.Env = append(os.Environ(), "SVAROG_CONN_STRING="+connectionURL)
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			require.Error(t, command.Run(), "invalid local configuration must fail startup")
			require.Empty(t, stdout.String(), "invalid local configuration must never announce readiness")
			require.NotEmpty(t, stderr.String(), "startup failure must explain the configuration error on stderr")
		}

		at := time.Date(2026, 11, 16, 10, 0, 0, 123456789, time.UTC)
		message := "Java \"message\" čćž\nsecond line"
		payload, err := json.Marshal(map[string]any{"message": message, "timestamp": at})
		require.NoError(t, err, "encode a Java-style input record with its original timestamp")

		for _, test := range []struct {
			name          string
			suffix        string
			noAck         bool
			expectedError string
		}{
			{name: "EOF confirms accepted records"},
			{name: "invalid JSON still drains earlier records", suffix: "not-json\n", expectedError: "decode log entry"},
			{name: "missing timestamp is not replaced", suffix: "{\"message\":\"missing timestamp\"}\n", expectedError: "timestamp is required"},
			{name: "unconfirmed publication fails the drain", noAck: true, expectedError: "shutdown interrupted"},
		} {
			t.Run(test.name, func(t *testing.T) {
				connectionURL, stream, _ := broker.stream(t, jetstream.StreamConfig{NoAck: test.noAck})
				ctx, cancel := context.WithTimeout(broker.ctx, 15*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, binary)
				command.Env = append(os.Environ(),
					"SVAROG_CONN_STRING="+connectionURL,
					"SVAROG_INSTANCE_ID=java-bridge",
				)
				input, err := command.StdinPipe()
				require.NoError(t, err, "open the bridge input without sending any logs yet")
				output, err := command.StdoutPipe()
				require.NoError(t, err, "read the startup response separately from log input")
				var stderr bytes.Buffer
				command.Stderr = &stderr
				require.NoError(t, command.Start(), "start the real bridge before sending logs")
				t.Cleanup(func() {
					_ = input.Close()
					cancel()
					if command.ProcessState == nil {
						_ = command.Wait()
					}
				})
				stdout := bufio.NewReader(output)
				ready, err := stdout.ReadString('\n')
				require.NoError(t, err, "readiness must arrive before the first log or EOF")
				require.Equal(t, "{\"v\":1,\"op\":\"ready\"}\n", ready, "readiness must identify the supported local startup protocol")
				_, err = io.WriteString(input, string(payload)+"\n"+test.suffix)
				require.NoError(t, err, "send source records only after the helper announces readiness")
				require.NoError(t, input.Close(), "EOF must initiate the publisher drain")
				remaining, err := io.ReadAll(stdout)
				require.NoError(t, err, "read the rest of stdout before waiting for the helper")
				err = command.Wait()
				if test.expectedError == "" {
					require.NoError(t, err, "EOF must drain accepted records before a successful exit: %s", stderr.String())
				} else {
					require.Error(t, err, "input or drain failures must produce an unsuccessful process exit")
					require.Contains(t, stderr.String(), test.expectedError, "diagnostics must explain the input or drain failure")
				}
				require.Empty(t, remaining, "the helper must emit exactly one ready response and no per-log acknowledgements")

				info, err := stream.Info(broker.ctx)
				require.NoError(t, err, "inspect the real broker after the helper has exited")
				require.EqualValues(t, 1, info.State.Msgs, "accepted records must survive input failure, without inventing a timestamp for rejected input")
				stored, err := stream.GetMsg(broker.ctx, 1)
				require.NoError(t, err, "read the accepted source record from JetStream")
				var line rpc.LogLine
				require.NoError(t, json.Unmarshal(stored.Data, &line), "the bridge must publish the existing RPC format")
				require.Equal(t, message, line.Message, "source Unicode, quotes and embedded newlines must survive publication")
				require.True(t, at.Equal(line.Timestamp), "the bridge must preserve the source timestamp, including nanoseconds")
				require.Equal(t, "java-bridge", line.InstanceId, "the shared client must resolve the instance from the inherited environment")
			})
		}
	})

	t.Run("the window applies backpressure and shutdown waits for every acknowledgement", func(t *testing.T) {
		connectionURL, stream, subject := broker.stream(t, jetstream.StreamConfig{NoAck: true})
		subscription := broker.observePublications(t, subject)
		client := newClient(t, connectionURL)

		window := make([]*nats.Msg, 32)
		for i := range window {
			require.NoError(t, client.Enqueue(svarogclient.Entry{Message: "in flight"}), "admit each record in the publication window")
			window[i] = nextPublication(t, subscription)
		}

		for range 16 {
			require.NoError(t, client.Enqueue(svarogclient.Entry{Message: "queued"}), "accept exactly sixteen entries beyond the full window")
		}
		blocked := make(chan error, 1)
		go func() { blocked <- client.Enqueue(svarogclient.Entry{Message: "waiting for space"}) }()
		select {
		case err := <-blocked:
			t.Fatalf("a full window and queue must block admission; returned %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		broker.acknowledge(t, stream, window[0])
		select {
		case err := <-blocked:
			require.NoError(t, err, "one acknowledgement must free a window slot and let a blocked producer proceed")
		case <-time.After(time.Second):
			t.Fatal("the window must advance without waiting for all its acknowledgements")
		}

		ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- client.Shutdown(ctx) }()
		for _, message := range window[1:] {
			broker.acknowledge(t, stream, message)
		}
		// Hold the penultimate acknowledgement while delivering the last one.
		for range 15 {
			broker.acknowledge(t, stream, nextPublication(t, subscription))
		}
		held := nextPublication(t, subscription)
		broker.acknowledge(t, stream, nextPublication(t, subscription))
		select {
		case err := <-done:
			t.Fatalf("shutdown must wait for the outstanding acknowledgement; returned %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		broker.acknowledge(t, stream, held)
		select {
		case err := <-done:
			require.NoError(t, err, "shutdown must succeed once every accepted entry is acknowledged")
		case <-ctx.Done():
			t.Fatal("shutdown must finish after the final outstanding acknowledgement")
		}
	})

	t.Run("lost acknowledgements retry only unconfirmed entries in order", func(t *testing.T) {
		connectionURL, stream, subject := broker.stream(t, jetstream.StreamConfig{NoAck: true})
		subscription := broker.observePublications(t, subject)
		client := newClient(t, connectionURL)
		// Retry diagnostics must stay local even while global logging uses this client.
		installLogger(t, client)

		for range 3 {
			require.NoError(t, client.Enqueue(svarogclient.Entry{Message: "original"}), "admit a sequence of entries before any of them is acknowledged")
		}
		first := nextPublication(t, subscription)
		confirmed := nextPublication(t, subscription)
		third := nextPublication(t, subscription)
		broker.acknowledge(t, stream, confirmed)
		require.Eventually(t, func() bool {
			info, err := stream.Info(broker.ctx)
			return err == nil && info.State.Msgs == 3
		}, 3*time.Second, 10*time.Millisecond, "the broker must store all first attempts before real acknowledgements are restored")

		info, err := stream.Info(broker.ctx)
		require.NoError(t, err, "read the original stream configuration")
		config := info.Config
		config.NoAck = false
		_, err = broker.js.UpdateStream(broker.ctx, config)
		require.NoError(t, err, "restore acknowledgements without deleting stored messages")
		for _, original := range []*nats.Msg{first, third} {
			retried := nextPublication(t, subscription)
			require.Equal(t, original.Data, retried.Data, "retry must preserve exact data and the admission order of unconfirmed entries")
			require.Equal(t, original.Header.Get(jetstream.MsgIDHeader), retried.Header.Get(jetstream.MsgIDHeader), "a lost acknowledgement must preserve event identity")
		}
		shutdownClient(t, client)
		_, err = subscription.NextMsg(100 * time.Millisecond)
		require.ErrorIs(t, err, nats.ErrTimeout, "the already acknowledged entry and retry diagnostics must not be published")
		info, err = stream.Info(broker.ctx)
		require.NoError(t, err, "read final retained messages after a successful drain")
		require.EqualValues(t, 3, info.State.Msgs, "broker deduplication must retain exactly one copy of each accepted event")
	})

	t.Run("reconnect resends the whole unconfirmed window after an immediate send failure", func(t *testing.T) {
		connectionURL, stream, subject := broker.stream(t, jetstream.StreamConfig{NoAck: true})
		subscription := broker.observePublications(t, subject)
		proxyURL, firstConnection, allowReconnect := broker.interruptibleProxy(t, connectionURL)
		reconnectFailed := make(reconnectDiagnostics, 1)
		client, err := svarogclient.New(svarogclient.Options{ConnectionString: proxyURL, Diagnostics: reconnectFailed})
		require.NoError(t, err, "start the client through the interruptible proxy")
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = client.Shutdown(ctx)
		})

		originals := make([]*nats.Msg, 4)
		for i := range originals {
			require.NoError(t, client.Enqueue(svarogclient.Entry{Message: "before disconnect"}), "admit multiple entries without a broker acknowledgement")
			originals[i] = nextPublication(t, subscription)
		}
		require.Eventually(t, func() bool {
			info, err := stream.Info(broker.ctx)
			return err == nil && info.State.Msgs == uint64(len(originals))
		}, 3*time.Second, 10*time.Millisecond, "store the whole window while acknowledgements are disabled")
		config := stream.CachedInfo().Config
		config.NoAck = false
		_, err = broker.js.UpdateStream(broker.ctx, config)
		require.NoError(t, err, "enable acknowledgements for publications made after reconnect")

		require.NoError(t, (<-firstConnection).Close(), "drop the TCP connection with all four publications still unconfirmed")
		select {
		case <-reconnectFailed:
			close(allowReconnect)
		case <-time.After(8 * time.Second):
			t.Fatal("the first resend must fail immediately while the reconnect buffer is disabled and the proxy rejects new connections")
		}
		for _, original := range originals {
			retried := nextPublication(t, subscription)
			require.Equal(t, original.Data, retried.Data, "resend every unconfirmed entry in admission order with its original bytes")
			require.Equal(t, original.Header.Get(jetstream.MsgIDHeader), retried.Header.Get(jetstream.MsgIDHeader), "preserve the original event identity after reconnect")
		}
		shutdownClient(t, client)
		info, err := stream.Info(broker.ctx)
		require.NoError(t, err, "read the stream after every accepted entry has been acknowledged")
		require.EqualValues(t, len(originals), info.State.Msgs, "reconnect and retry must retain exactly one copy of every accepted entry")
		for i, original := range originals {
			stored, err := stream.GetMsg(broker.ctx, uint64(i+1))
			require.NoError(t, err, "each accepted entry must remain readable after reconnect")
			require.Equal(t, original.Data, stored.Data, "deduplication must retain the original payload at each sequence")
		}
	})

	t.Run("drop mode bounds the window and queue by encoded bytes and counts rejections", func(t *testing.T) {
		connectionURL, _, subject := broker.stream(t, jetstream.StreamConfig{NoAck: true})
		subscription := broker.observePublications(t, subject)
		diagnostics, err := os.Create(filepath.Join(t.TempDir(), "diagnostics.log"))
		require.NoError(t, err, "capture diagnostics without sharing a mutable buffer with the publisher")
		t.Cleanup(func() { _ = diagnostics.Close() })
		client, err := svarogclient.New(svarogclient.Options{
			ConnectionString: connectionURL, DropQueueCapacity: 20, Diagnostics: diagnostics,
		})
		require.NoError(t, err, "create a best-effort sender with diagnostic output captured separately")
		t.Cleanup(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_ = client.Shutdown(ctx)
		})

		// JSON escapes '<' to six bytes. Four payloads fit in 4 MiB,
		// and sixteen fit in the queue's separate 16 MiB budget.
		large := svarogclient.Entry{Message: strings.Repeat("<", 170_000)}
		for range 4 {
			require.NoError(t, client.Enqueue(large), "admit one large entry into the window")
			nextPublication(t, subscription)
		}
		for range 16 {
			require.NoError(t, client.Enqueue(large), "the queue must retain sixteen near-limit payloads")
		}
		require.ErrorIs(t, client.Enqueue(large), svarogclient.ErrQueueFull, "encoded bytes must bound admission even when queue slots remain")
		for range 4 {
			require.NoError(t, client.Enqueue(svarogclient.Entry{Message: "small"}), "a byte-limit rejection must leave free slots available for smaller records")
		}
		for range 100 {
			require.ErrorIs(t, client.Enqueue(svarogclient.Entry{Message: "extra"}), svarogclient.ErrQueueFull, "a full drop-mode queue must reject every excess record")
		}
		// The next eligible rejection makes the accumulated count visible.
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		<-timer.C
		require.ErrorIs(t, client.Enqueue(svarogclient.Entry{Message: "later"}), svarogclient.ErrQueueFull, "the queue must stay full without acknowledgements")
		for range 7 {
			require.ErrorIs(t, client.Enqueue(svarogclient.Entry{Message: "before shutdown"}), svarogclient.ErrQueueFull, "count a final burst inside the reporting interval")
		}
		_, err = subscription.NextMsg(100 * time.Millisecond)
		require.ErrorIs(t, err, nats.ErrTimeout, "the byte limit must prevent a fifth large publication while acknowledgements are outstanding")
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		require.ErrorIs(t, client.Shutdown(ctx), context.DeadlineExceeded, "shutdown must report that in-flight and queued entries could not drain")
		report, err := os.ReadFile(diagnostics.Name())
		require.NoError(t, err, "read the diagnostic summaries after shutdown")
		require.Equal(t, 3, bytes.Count(report, []byte("Log entries were not accepted")), "only the first, interval and shutdown summaries must be emitted")
		require.Contains(t, string(report), "dropped=101", "the next report must include rejections suppressed in the preceding interval")
		require.Contains(t, string(report), "dropped=7 ", "shutdown must report the final burst without another rejection after the interval")
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

func TestLoggingSetupLifecycle(t *testing.T) {
	preserveLogger(t)
	broker := newClientBroker(t)

	t.Run("default logging drops excess records without blocking the console", func(t *testing.T) {
		connectionURL, _, subject := broker.stream(t, jetstream.StreamConfig{NoAck: true})
		subscription := broker.observePublications(t, subject)
		var console bytes.Buffer
		closeLogging, err := svaroglog.Setup(t.Context(),
			svaroglog.WithConnectionString(connectionURL),
			svaroglog.WithConsole(slog.NewTextHandler(&console, nil)), svaroglog.WithDrainTimeout(100*time.Millisecond))
		require.NoError(t, err, "setup must start with best-effort forwarding by default")
		t.Cleanup(func() { _ = closeLogging() })

		logger := slog.Default()
		for range 32 {
			logger.Info("in flight")
			nextPublication(t, subscription)
		}
		produced := make(chan error, 1)
		go func() {
			handler := logger.Handler()
			for range 1024 {
				if err := handler.Handle(t.Context(), slog.NewRecord(time.Now(), slog.LevelInfo, "accepted", 0)); err != nil {
					produced <- err
					return
				}
			}
			produced <- handler.Handle(t.Context(), slog.NewRecord(time.Now(), slog.LevelInfo, "dropped", 0))
		}()
		select {
		case err := <-produced:
			require.ErrorIs(t, err, svarogclient.ErrQueueFull, "the default queue must drop excess logs instead of blocking")
		case <-time.After(2 * time.Second):
			t.Fatal("a full default logging queue must never wait for the broker")
		}
		require.Contains(t, console.String(), "msg=dropped", "the dropped record must still reach the configured console")
		require.ErrorIs(t, closeLogging(), context.DeadlineExceeded, "the deadline must report accepted entries that were not acknowledged")
	})

	t.Run("environment setup drains and restores derived global loggers", func(t *testing.T) {
		connectionURL, stream, _ := broker.stream(t, jetstream.StreamConfig{})
		t.Setenv("SVAROG_CONN_STRING", connectionURL)
		previous := slog.New(slog.NewTextHandler(io.Discard, nil))
		slog.SetDefault(previous)
		log.SetFlags(log.Lshortfile)
		log.SetPrefix("previous: ")
		output, flags, prefix := log.Writer(), log.Flags(), log.Prefix()
		slog.SetLogLoggerLevel(slog.LevelWarn)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		closeLogging, err := svaroglog.Setup(ctx, svaroglog.WithInstanceID("setup-instance"), svaroglog.WithLevel(slog.LevelDebug), svaroglog.WithConsole(nil))
		require.NoError(t, err, "environment configuration must be sufficient to install the logging integration")
		t.Cleanup(func() { _ = closeLogging() })

		installed := slog.Default()
		_, err = svaroglog.Setup(ctx)
		require.ErrorIs(t, err, svaroglog.ErrAlreadyConfigured, "a second global setup must not start another publisher")
		require.Same(t, installed, slog.Default(), "a rejected setup must leave the active logger untouched")
		slog.SetDefault(slog.Default().With("service", "api").WithGroup("request"))
		slog.Debug("structured entry", "id", 42)
		log.Printf("standard entry")
		cancel()
		require.Eventually(t, func() bool {
			return slog.Default() == previous
		}, 5*time.Second, 10*time.Millisecond, "context cancellation must independently drain and restore even a derived global logger")
		require.NoError(t, closeLogging(), "the already-canceled lifetime must not be reused as the drain deadline")
		require.NoError(t, closeLogging(), "repeated cleanup must preserve successful completion")
		require.Equal(t, output, log.Writer(), "cleanup must restore the previous standard log destination")
		require.Equal(t, flags, log.Flags(), "cleanup must restore the previous standard log flags")
		require.Equal(t, prefix, log.Prefix(), "cleanup must restore the previous standard log prefix")
		require.Equal(t, slog.LevelWarn, slog.SetLogLoggerLevel(slog.LevelInfo), "cleanup must restore the previous standard log bridge level")

		info, err := stream.Info(broker.ctx)
		require.NoError(t, err, "read the acknowledged logs after automatic shutdown")
		require.EqualValues(t, 2, info.State.Msgs, "global slog and standard log must each publish through the owned client")
		for sequence := uint64(1); sequence <= 2; sequence++ {
			message, err := stream.GetMsg(broker.ctx, sequence)
			require.NoError(t, err, "each accepted log must be available after the automatic drain")
			var line rpc.LogLine
			require.NoError(t, json.Unmarshal(message.Data, &line), "setup must preserve the existing wire payload")
			require.Equal(t, "setup-instance", line.InstanceId, "the setup instance option must configure the shared publisher")
			require.Contains(t, line.Message, "service=api", "both logging APIs must preserve derived global attributes")
		}
	})

	t.Run("lifetime cancellation releases a full queue without waiting for caller cleanup", func(t *testing.T) {
		connectionURL, _, subject := broker.stream(t, jetstream.StreamConfig{NoAck: true})
		t.Setenv("SVAROG_CONN_STRING", "invalid-environment-url")
		subscription, err := broker.nc.SubscribeSync(subject)
		require.NoError(t, err, "observe when the unavailable acknowledgement starts blocking publication")
		defer subscription.Unsubscribe()
		require.NoError(t, broker.nc.Flush(), "observe the first message before filling the queue")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		closeLogging, err := svaroglog.Setup(ctx, svaroglog.WithConnectionString(connectionURL), svaroglog.WithDropQueueCapacity(0), svaroglog.WithConsole(nil), svaroglog.WithDrainTimeout(100*time.Millisecond))
		require.NoError(t, err, "an explicit connection URL must override the environment without waiting for broker acknowledgements")
		t.Cleanup(func() { _ = closeLogging() })
		logger := slog.Default()
		for range 32 {
			logger.Info("in flight")
			nextPublication(t, subscription)
		}
		for range 16 {
			logger.Info("queued")
		}
		blocked := make(chan struct{})
		go func() {
			logger.Info("waiting for admission")
			close(blocked)
		}()
		select {
		case <-blocked:
			t.Fatal("logging must apply backpressure while acknowledgements are unavailable and the queue is full")
		case <-time.After(100 * time.Millisecond):
		}
		cancel()
		select {
		case <-blocked:
		case <-time.After(time.Second):
			t.Fatal("lifetime cancellation must release the application without requiring that application to call cleanup first")
		}
		results := make(chan error, 4)
		for range 4 {
			go func() { results <- closeLogging() }()
		}
		for range 4 {
			select {
			case err := <-results:
				require.ErrorIs(t, err, context.DeadlineExceeded, "concurrent cleanup must all observe the bounded failed drain, not lifetime cancellation")
			case <-time.After(time.Second):
				t.Fatal("concurrent cleanup must join automatic shutdown within its deadline")
			}
		}
		require.ErrorIs(t, closeLogging(), context.DeadlineExceeded, "later cleanup must retain the same failed drain result")
	})

	t.Run("cleanup does not overwrite an independently replaced global logger", func(t *testing.T) {
		connectionURL, _, _ := broker.stream(t, jetstream.StreamConfig{})
		closeLogging, err := svaroglog.Setup(context.Background(), svaroglog.WithConnectionString(connectionURL), svaroglog.WithConsole(nil))
		require.NoError(t, err, "a completed setup must release ownership for a subsequent installation")
		t.Cleanup(func() { _ = closeLogging() })
		replacement := slog.New(slog.NewTextHandler(io.Discard, nil))
		slog.SetDefault(replacement)
		log.SetFlags(log.Lmsgprefix)
		log.SetPrefix("replacement: ")
		output := log.Writer()
		slog.SetLogLoggerLevel(slog.LevelError)
		require.NoError(t, closeLogging(), "an idle owned publisher must close without affecting its replacement")
		require.Same(t, replacement, slog.Default(), "cleanup must leave the application's newer global logger installed")
		require.Equal(t, output, log.Writer(), "cleanup must preserve the newer standard log bridge")
		require.Equal(t, log.Lmsgprefix, log.Flags(), "cleanup must preserve the replacement logger's flags")
		require.Equal(t, "replacement: ", log.Prefix(), "cleanup must preserve the replacement logger's prefix")
		require.Equal(t, slog.LevelError, slog.SetLogLoggerLevel(slog.LevelInfo), "cleanup must preserve the replacement logger's bridge level")
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

func (b clientBroker) observePublications(t *testing.T, subject string) *nats.Subscription {
	t.Helper()
	subscription, err := b.nc.SubscribeSync(subject)
	require.NoError(t, err, "observe publish attempts on the real NATS broker")
	t.Cleanup(func() { _ = subscription.Unsubscribe() })
	require.NoError(t, b.nc.Flush(), "install observation before the first publish")
	return subscription
}

// The initial connection is observable and later connections are rejected until
// the test has seen the publisher try to resend with the reconnect buffer off.
func (b clientBroker) interruptibleProxy(t *testing.T, connectionURL string) (string, <-chan net.Conn, chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "listen on loopback for the client's broker traffic")
	t.Cleanup(func() { _ = listener.Close() })
	proxyURL, err := url.Parse(connectionURL)
	require.NoError(t, err, "reuse the generated subject and credentials through the proxy")
	proxyURL.Host = listener.Addr().String()
	firstConnection := make(chan net.Conn, 1)
	allowReconnect := make(chan struct{})
	relay := func(dst, src net.Conn) {
		defer dst.Close()
		defer src.Close()
		_, _ = io.Copy(dst, src)
	}
	go func() {
		for first := true; ; first = false {
			incoming, err := listener.Accept()
			if err != nil {
				return
			}
			if !first {
				select {
				case <-allowReconnect:
				default:
					_ = incoming.Close()
					continue
				}
			}
			upstream, err := net.DialTimeout("tcp", b.host, time.Second)
			if err != nil {
				_ = incoming.Close()
				continue
			}
			go relay(upstream, incoming)
			go relay(incoming, upstream)
			if first {
				firstConnection <- incoming
			}
		}
	}()
	return proxyURL.String(), firstConnection, allowReconnect
}

type reconnectDiagnostics chan struct{}

func (d reconnectDiagnostics) Write(line []byte) (int, error) {
	if bytes.Contains(line, []byte("Retrying unconfirmed log publications")) &&
		bytes.Contains(line, []byte(nats.ErrReconnectBufExceeded.Error())) {
		select {
		case d <- struct{}{}:
		default:
		}
	}
	return len(line), nil
}

func nextPublication(t *testing.T, subscription *nats.Subscription) *nats.Msg {
	t.Helper()
	msg, err := subscription.NextMsg(8 * time.Second)
	require.NoError(t, err, "the expected publication must reach NATS")
	return msg
}

// The NoAck stream stores messages but leaves each reply under test control.
func (b clientBroker) acknowledge(t *testing.T, stream jetstream.Stream, msg *nats.Msg) {
	t.Helper()
	require.NotEmpty(t, msg.Reply, "the publication must provide a reply subject for its acknowledgement")
	require.NoError(t, b.nc.Publish(msg.Reply, fmt.Appendf(nil, `{"stream":%q,"seq":1}`, stream.CachedInfo().Config.Name)),
		"send an acknowledgement only for the chosen publication")
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
	preserveLogger(t)
	slog.SetDefault(slog.New(svarogclient.NewHandler(client, svarogclient.HandlerOptions{Level: slog.LevelDebug})))
}

func preserveLogger(t *testing.T) {
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
