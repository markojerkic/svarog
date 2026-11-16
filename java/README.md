# Java logging demo

Requires JDK 17+, Go from the repository's `go.mod`, a running Svarog/NATS
environment and a generated client connection URL. Maven is provided by the wrapper.

- `svarog-logback`: a minimal Logback appender that starts one Go helper and
  writes JSON lines to its stdin.
- `svarog-demo`: an application that depends on the library and calls SLF4J.

From the repository root:

```sh
go build -o /tmp/opencode/svarog-bridge ./cmd/svarog-bridge
./java/mvnw -f java/pom.xml install

export SVAROG_CONN_STRING='PASTE_GENERATED_CONNECTION_URL'
export SVAROG_INSTANCE_ID='java-demo' # Optional.
SVAROG_BRIDGE_PATH=/tmp/opencode/svarog-bridge \
  ./java/mvnw -f java/svarog-demo/pom.xml exec:java
```

The helper inherits `SVAROG_CONN_STRING` and `SVAROG_INSTANCE_ID` from the Java
process. It forwards the message and original logging timestamp through the
shared Go publisher to NATS JetStream. Messages are no longer printed locally:
look for `Hello from svarog-demo!` in Svarog after ingestion.

Flow: SLF4J → Logback → `DemoAppender` → JSON over stdin → Go helper →
`svarogclient` → NATS JetStream.

The demo explicitly stops its Logback context in `finally`, which stops the
appender, closes stdin (EOF), and waits for the helper to exit before Maven closes
the application classloader. The helper gives the shared publisher up to four
seconds to obtain broker acknowledgements. This is not a PostgreSQL persistence
confirmation. Helper stderr is inherited; stdout is discarded.
`SVAROG_BRIDGE_PATH` must point to the built executable, not a `go run` command.

This is still a synchronous prototype: no IPC acknowledgements, automatic restart
or handling of stalled pipe writes. Publication retries and bounded buffering are
provided by the shared Go publisher. Helper failures are reported locally by the
appender, not returned by SLF4J logging calls or reflected in Maven's build status.
The appender's five-second exit wait starts after stdin is closed; it is not a
total shutdown deadline.
