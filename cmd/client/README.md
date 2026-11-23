# Svarog CLI client

`svarog-client` reads log lines from stdin and forwards them to Svarog. Use it
for any program whose output can be piped. The binaries are static, so running
them does not require Docker or Go.

For JSON entries with their own timestamps, use the
[bridge](../svarog-bridge/README.md). Go and Java applications can send logs
directly through the [Go library](../../svaroglog/README.md) or the
[Java appender](../../java/README.md).

- [Download and run](#download-and-run)
- [Docker image](#docker-image)
- [Configuration](#configuration)
- [Behavior](#behavior)
- [Build locally](#build-locally)

## Download and run

Linux amd64:

```bash
curl -fsSL https://github.com/markojerkic/svarog/releases/latest/download/svarog-client_linux_amd64.tar.gz | tar -xz
your-app 2>&1 | ./svarog-client "PASTE_GENERATED_CONNECTION_URL"
```

Copy the connection URL from the Svarog web UI. For Linux arm64, replace
`amd64` with `arm64` in the download URL.

To pin a version, replace `/releases/latest/download/<filename>` with
`/releases/download/vX.Y.Z/<filename>`. Each
[release](https://github.com/markojerkic/svarog/releases) includes one
`checksums.txt` for its binaries.

## Docker image

`ghcr.io/markojerkic/svarog-client` contains a static `linux/amd64` binary at
`/svarog/client`. Copy it into your application image:

```Dockerfile
FROM ghcr.io/markojerkic/svarog-client:latest AS svarog

FROM alpine:3.24.3
COPY --from=svarog /svarog/client /usr/local/bin/svarog-client
COPY ./your-app /app/your-app

CMD ["sh", "-o", "pipefail", "-c", "/app/your-app 2>&1 | svarog-client"]
```

Pass the connection URL through `SVAROG_CONN_STRING`. To let the client drain on
`docker stop`, run the container with Docker's init process (`docker run --init`
or `init: true` in Compose) and set `TINI_KILL_PROCESS_GROUP=1`. The stop signal
then reaches every process in the pipeline. The
[log demo](../log-demo/compose.yml) uses this setup.

## Configuration

| Setting              | Default  | Purpose                                                     |
| -------------------- | -------- | ----------------------------------------------------------- |
| First argument       | Unset    | Connection URL. Takes precedence over `SVAROG_CONN_STRING`. |
| `SVAROG_CONN_STRING` | Unset    | Connection URL when no argument is passed.                  |
| `SVAROG_INSTANCE_ID` | Hostname | Instance name attached to the logs.                         |

Append `&debug=true` to the connection URL for verbose diagnostics on stderr.

## Behavior

- Each line is printed to stdout unchanged and forwarded without ANSI styling.
  It is timestamped when the client reads it.
- Lines longer than 512 KiB are truncated and end with ` [truncated]`. Lines
  that are not valid UTF-8 or contain NUL characters are not forwarded.
- The client sends up to 32 records and 4 MiB of encoded data at a time while
  waiting for JetStream acknowledgements. When the separate 16-slot queue fills,
  it stops reading stdin, which applies backpressure to the input pipe.
- On EOF, `SIGINT` or `SIGTERM`, it waits up to 30 seconds for JetStream to
  acknowledge accepted logs. It exits with an error if the drain fails.

## Build locally

From the repository root, using the Go version in `go.mod`:

```bash
task build:client
```

The task calls `scripts/build-binaries.sh` and writes:

```text
build/client/
├── svarog-client_linux_amd64.tar.gz
├── svarog-client_linux_arm64.tar.gz
└── checksums.txt
```

Use `task build:client OUT=/path/to/output` to select another output directory.
Builds use `CGO_ENABLED=0`, so the binaries do not depend on a system C library.

To build only the binary for the current platform:

```bash
go build -o svarog-client ./cmd/client
tail -F /var/log/app.log | ./svarog-client "$SVAROG_CONN_STRING"
```
