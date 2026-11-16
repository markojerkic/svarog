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
