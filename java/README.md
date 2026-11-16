# Java logging demo

Requires JDK 17+ and Go from the repository's `go.mod`. Maven is provided by the
wrapper.

- `svarog-logback`: a minimal Logback appender that starts one Go helper and
  writes JSON lines to its stdin.
- `svarog-demo`: an application that depends on the library and calls SLF4J.

From the repository root:

```sh
go build -o /tmp/opencode/svarog-bridge ./cmd/svarog-bridge
./java/mvnw -f java/pom.xml install
SVAROG_BRIDGE_PATH=/tmp/opencode/svarog-bridge \
  ./java/mvnw -f java/svarog-demo/pom.xml exec:java
```

The Go helper prints the message and the original logging timestamp to stderr,
for example:

```text
timestamp=2026-11-16T10:00:00.123456789Z message="Hello from svarog-demo!"
```

Flow: SLF4J → Logback → `DemoAppender` → JSON over stdin → Go helper.

The demo explicitly stops its Logback context in `finally`, which stops the
appender, closes stdin (EOF), and waits for the helper to exit before Maven closes
the application classloader. Helper stderr is inherited; stdout is discarded.
`SVAROG_BRIDGE_PATH` must point to the built executable, not a `go run` command.

This is still a synchronous input/output prototype: no NATS, acknowledgements,
retry, automatic restart or handling of stalled pipe writes. The five-second
exit wait starts after stdin is closed; it is not a total shutdown deadline.
