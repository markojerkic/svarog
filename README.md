# Svarog

Svarog collects application logs over NATS, stores them in PostgreSQL and shows
them in a web UI.

- Projects with clients, each sending logs from one or more named instances.
- Live tailing, substring search, date ranges and context around a log line.
- Scheduled and manual ZIP archiving per client, on disk or in S3-compatible storage.
- On-demand search of archived logs: open **Archiving → Search archived logs** as
  an administrator, choose a local time range and optionally enter message text.
  The end time is exclusive. Search verifies the matching ZIPs and returns the
  newest 200 matches and the total count, without importing records into PostgreSQL.
  Each request scans at most 128 ZIPs, 256 MiB compressed, 512 MiB expanded and
  8 MiB of result text, with a one-minute scan timeout. If a limit is exceeded or
  a ZIP cannot be verified, no partial results are shown.
- User accounts with per-project access.

Applications pipe their output into the Svarog client, which publishes it to
NATS JetStream; the server stores it in PostgreSQL and streams new lines to the UI.

## Server

- `ghcr.io/markojerkic/svarog` connects to an external NATS broker and supports multiple replicas.
- `ghcr.io/markojerkic/svarog-embedded` runs NATS JetStream in the server process and supports exactly one replica.

Both images are published with `vX.Y.Z` and `latest` tags and use PostgreSQL.

### External NATS

[docker-compose.prod.yml](docker-compose.prod.yml) runs the server and an
authenticated NATS server. PostgreSQL is not included and must already exist.

#### NATS credentials

From a checkout of this repository, run:

```bash
go run ./cmd/nats-setup
```

It prints values for `.env` and a configuration to save as
`$SVAROG_DEPLOY_DIR/nats-server.conf`. Append JetStream storage to that file:

```conf
jetstream {
  store_dir: /data/jetstream
}
```

### Configuration

Create `.env`:

```dotenv
SVAROG_IMAGE=ghcr.io/markojerkic/svarog:latest
SVAROG_DEPLOY_DIR=/absolute/path/to/svarog
# host.docker.internal is the Docker host; PostgreSQL must accept connections from containers
POSTGRES_URL=postgres://svarog:password@host.docker.internal:5432/svarog
SESSION_SECRET=replace-with-a-long-random-secret
NATS_PUBLIC_ADDR=logs.example.com:4222
# Printed by nats-setup
NATS_ACCOUNT_SEED=...
NATS_SERVER_USER_JWT=...
NATS_SERVER_USER_SEED=...
```

Server and deployment defaults below are for external-NATS production Compose.
Set any additional server settings explicitly in the `svarog` service's
`environment` in `docker-compose.prod.yml`. The development
stack uses `.env`; the demo stack uses `.env.log-demo`.

| Variable                                                                  | Default                              | Purpose                                                                                                                                                                                                                                         |
| ------------------------------------------------------------------------- | ------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Deployment and networking**                                             |                                      |                                                                                                                                                                                                                                                 |
| `SVAROG_IMAGE`                                                            | Required                             | Server image to deploy. Use a `vX.Y.Z` tag instead of `latest` to pin the version.                                                                                                                                                              |
| `SVAROG_DEPLOY_DIR`                                                       | Required                             | Absolute host directory containing `nats-server.conf`.                                                                                                                                                                                          |
| `POSTGRES_URL`                                                            | Required                             | Direct PostgreSQL connection URL (no transaction-pooling proxy). The database user must be able to create `pg_trgm` unless already installed.                                                                                                   |
| `SESSION_SECRET`                                                          | Required                             | Secret used to sign session cookies.                                                                                                                                                                                                            |
| `PUBLIC_URL`                                                              | Unset                                | Public origin of the web UI, e.g. `https://logs.example.com`. Enables the [MCP endpoint](#mcp) at `/mcp`. Must use HTTPS unless it points to localhost.                                                                                         |
| `NATS_PUBLIC_ADDR`                                                        | Required                             | Host and port written into client connection URLs, e.g. `logs.example.com:4222`.                                                                                                                                                                |
| `NATS_ACCOUNT_SEED`                                                       | Required                             | Account seed used to sign client credentials. Generated by `nats-setup`.                                                                                                                                                                        |
| `NATS_SERVER_USER_JWT`                                                    | Required                             | NATS user JWT for the server. Generated by `nats-setup`.                                                                                                                                                                                        |
| `NATS_SERVER_USER_SEED`                                                   | Required                             | NATS user seed paired with the server JWT. Generated by `nats-setup`.                                                                                                                                                                           |
| `NATS_ACCOUNT_PUBLIC_KEY`                                                 | Unset                                | Public key printed by `nats-setup` for reference; the server does not require it.                                                                                                                                                               |
| `SVAROG_HTTP_BIND`                                                        | `127.0.0.1`                          | Host address on which Compose publishes the web UI.                                                                                                                                                                                             |
| `SVAROG_HTTP_PORT`                                                        | `1323`                               | Host port for the web UI.                                                                                                                                                                                                                       |
| `SVAROG_NATS_BIND`                                                        | `0.0.0.0`                            | Host address on which Compose publishes NATS.                                                                                                                                                                                                   |
| `SVAROG_NATS_PORT`                                                        | `4222`                               | Host port for NATS clients.                                                                                                                                                                                                                     |
| `HTTP_SERVER_PORT`                                                        | `1323`                               | Port the server listens on inside the container.                                                                                                                                                                                                |
| `NATS_ADDR`                                                               | `nats://nats:4222`                   | Internal broker URL the server connects to.                                                                                                                                                                                                     |
| `DOCKER`                                                                  | `true` in the server image           | Skip loading local `.env` files and use the container environment.                                                                                                                                                                              |
| `SERVER_DNS_NAME`                                                         | Unset                                | Parsed but currently unused by the server.                                                                                                                                                                                                      |
| **Server and client logging**                                             |                                      |                                                                                                                                                                                                                                                 |
| `LOG_LEVEL`                                                               | `info`                               | Minimum severity for server logs: `debug`, `info`, `warn` or `error`.                                                                                                                                                                           |
| `SVAROG_CONN_STRING`                                                      | Unset                                | **Server:** forward its own logs to this client URL; unset means console only. **CLI:** use this URL unless one is passed as the first argument. **Go library:** default URL. **Java bridge:** required.                                        |
| `SVAROG_INSTANCE_ID`                                                      | Hostname                             | Instance name attached to forwarded logs. The Go library can override it with `WithInstanceID`.                                                                                                                                                 |
| `SVAROG_LOG_QUEUE_CAPACITY`                                               | `1024`                               | Maximum records waiting per server process (1–65536; invalid values use the default). The queue also caps encoded data at 16 MiB, excluding the publication window. If either limit would be exceeded, the new record goes only to the console. |
| `SVAROG_BRIDGE_PATH`                                                      | Bundled executable                   | Absolute path to an alternative bridge executable for [Java Logback](java/README.md).                                                                                                                                                           |
| `SVAROG_LOGGING_ENABLED`                                                  | `false`                              | Enable log forwarding with the Svarog Spring Boot starter.                                                                                                                                                                                      |
| `TINI_KILL_PROCESS_GROUP`                                                 | Unset                                | Set to `1` when piping logs to the CLI under Tini so `docker stop` signals the whole pipeline.                                                                                                                                                  |
| **Archive storage and ingestion**                                         |                                      |                                                                                                                                                                                                                                                 |
| `ARCHIVE_DIR`                                                             | `/var/lib/svarog/archives`           | Directory for disk archives when S3 storage is not configured.                                                                                                                                                                                  |
| `ARCHIVE_SCHEDULER_ENABLED`                                               | `true`                               | Run archive policies automatically when archive storage is configured.                                                                                                                                                                          |
| `ARCHIVE_S3_BUCKET`                                                       | Unset                                | Destination bucket; required for S3 archive storage.                                                                                                                                                                                            |
| `ARCHIVE_S3_ENDPOINT`                                                     | Unset                                | S3-compatible endpoint URL; required for S3 archive storage.                                                                                                                                                                                    |
| `ARCHIVE_S3_REGION`                                                       | `auto`                               | Region used for S3 requests.                                                                                                                                                                                                                    |
| `ARCHIVE_S3_ACCESS_KEY_ID`                                                | Unset                                | Access key ID for S3 archive storage.                                                                                                                                                                                                           |
| `ARCHIVE_S3_SECRET_ACCESS_KEY`                                            | Unset                                | Secret key for S3 archive storage.                                                                                                                                                                                                              |
| `INGEST_STREAM_MAX_BYTES`                                                 | `0`                                  | JetStream log stream size limit in bytes. `0` keeps the existing limit or sets 1 GiB for a new stream.                                                                                                                                          |
| `INGEST_ACK_WAIT`                                                         | `60s`                                | How long JetStream waits for acknowledgement before a delivery is eligible for retry.                                                                                                                                                           |
| `INGEST_ALLOW_RETENTION_MIGRATION`                                        | `false`                              | Allow migration of a legacy log stream's retention policy after backing it up and stopping old writers.                                                                                                                                         |
| **Local development**                                                     |                                      |                                                                                                                                                                                                                                                 |
| `SVAROG_VERSION`                                                          | `local`                              | Server image tag for the development and load-balanced Compose stacks.                                                                                                                                                                          |
| `SVAROG_POSTGRES_PORT`                                                    | `5434`                               | Host port for the development PostgreSQL container.                                                                                                                                                                                             |
| `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`                       | `svarog` each                        | Database name, user and password set on the development PostgreSQL container in `docker-compose.dev.yml`.                                                                                                                                       |
| `PROXY_PORT`                                                              | `7331`                               | Port used by the development hot-reload proxy.                                                                                                                                                                                                  |
| `SHADCN_TEMPL_DEV`, `TEMPL_DEV_MODE`                                      | Unset                                | Either set to `true` disables browser caching of UI assets. `task dev` sets `SHADCN_TEMPL_DEV=true`.                                                                                                                                            |
| `GO_ENV`                                                                  | Unset                                | `development` is the legacy alias for disabling asset caching.                                                                                                                                                                                  |
| `GPRC_PORT`                                                               | Unused                               | Legacy entry in the development `.env`; it does not configure a listener.                                                                                                                                                                       |
| **Log demo**                                                              |                                      |                                                                                                                                                                                                                                                 |
| `API_CONN_STRING`, `WORKER_CONN_STRING`, `PAYMENTS_CONN_STRING`           | Required for demo Compose            | Client connection URLs for the API, worker and payments generators, respectively.                                                                                                                                                               |
| `NATIVE_CONN_STRING`                                                      | `API_CONN_STRING`                    | Client URL for the native Go logging demo.                                                                                                                                                                                                      |
| `SPRING_BOOT_CONN_STRING`                                                 | `API_CONN_STRING`                    | Client URL for the Spring Boot logging demo.                                                                                                                                                                                                    |
| `API_REPLICAS`, `WORKER_REPLICAS`, `PAYMENTS_REPLICAS`, `NATIVE_REPLICAS` | `3`, `2`, `2`, `1`                   | Number of generator containers for each demo service, respectively.                                                                                                                                                                             |
| `SPRING_BOOT_REPLICAS`                                                    | `1`                                  | Number of Spring Boot generator containers.                                                                                                                                                                                                     |
| `DEMO_NETWORK_MODE`                                                       | `host`                               | Docker network mode for demo containers.                                                                                                                                                                                                        |
| `DEMO_INTERVAL`                                                           | `1s`                                 | Time between generated log records.                                                                                                                                                                                                             |
| `DEMO_APP_NAME`                                                           | `demo` (CLI), `demo-native` (native) | Application name attached to generated logs. Demo Compose sets it per service.                                                                                                                                                                  |
| `DEMO_MESSAGES_FILE`                                                      | `cmd/log-demo/messages.txt`          | Message source for the CLI demo generator; the image uses `/app/messages.txt`.                                                                                                                                                                  |

Start the server:

```bash
docker compose --env-file .env -f docker-compose.prod.yml up -d --pull always
```

Migrations run on startup. A fresh database gets the user `admin` /
`ADMINADMIN`, who must change the password at first login.

The web UI listens on `127.0.0.1` by default. A reverse proxy in front of it
must forward WebSocket upgrades for live logs and, when it terminates TLS, send
`X-Forwarded-Proto: https` so session cookies are marked secure.

### MCP

With `PUBLIC_URL=https://logs.example.com`, AI agents can connect to the MCP
endpoint `https://logs.example.com/mcp`, for example with
`claude mcp add --transport http svarog https://logs.example.com/mcp`. Clients
must use exactly this URL, because OAuth tokens are bound to it.

The client opens a Svarog page in the browser, where the user signs in and
allows access. The client then receives an access token valid for one hour and
a refresh token valid for 30 days since its last use. It sees the same projects
as its user in the web UI. Changing the user's password or generating a new
login link disconnects all of the user's MCP clients.

Clients identify themselves with an OAuth Client ID Metadata Document, as the
MCP specification recommends. Clients that support only Dynamic Client
Registration cannot connect. Svarog fetches client metadata only over HTTPS
and only from public IP addresses.

### Embedded NATS

The embedded image uses the same PostgreSQL, session, archive and logging
settings. It generates the NATS operator and `$SYS` account on every start.
Use HTTP `/login` for health checks. The broker exposes only the NATS client
listener. `SVAROG_CONN_STRING` enables self-logging in both server modes. In
embedded mode it connects to the server's own broker, so its `host:port` must
be reachable from inside the container. When the published port differs from
`NATS_LISTEN_PORT`, as in the Compose example below, replace the copied URL's
`host:port` with `localhost:4222`.

| Variable            | Default                | Purpose                                                                                         |
| ------------------- | ---------------------- | ----------------------------------------------------------------------------------------------- |
| `NATS_ACCOUNT_SEED` | Required               | APP account seed from `nats-setup` or the existing deployment. Used to sign client credentials. |
| `NATS_PUBLIC_ADDR`  | Required               | Client-facing `host:port` written into generated connection URLs.                               |
| `NATS_LISTEN_HOST`  | `0.0.0.0`              | Address the embedded broker listens on.                                                         |
| `NATS_LISTEN_PORT`  | `4222`                 | Port the embedded broker listens on.                                                            |
| `NATS_STORE_DIR`    | `/var/lib/svarog/nats` | Persistent JetStream storage directory.                                                         |

The embedded image generates its broker configuration internally and omits
`NATS_ADDR`, `NATS_SERVER_USER_JWT`, `NATS_SERVER_USER_SEED` and `nats-server.conf`.

The embedded listener has no TLS option. Log contents travel in plaintext, so
keep the listener on a private network or behind an encrypted tunnel.

The broker accepts any user JWT signed by the APP account key. That includes
the `NATS_SERVER_USER_JWT` that `nats-setup` creates, which may publish and
subscribe to every subject and never expires. Keep it secret after switching to
the embedded image. Revocations and signing keys of a customized external
account are not carried over. For such an account, use a new
`NATS_ACCOUNT_SEED` and reissue client connection URLs.

[docker-compose.embedded.yml](docker-compose.embedded.yml) runs the embedded
image with its own PostgreSQL and volumes for JetStream and disk archives. Set
`SESSION_SECRET`, `NATS_ACCOUNT_SEED` and a URL-safe `POSTGRES_PASSWORD` in
`.env.embedded`:

```bash
docker compose --env-file .env.embedded -f docker-compose.embedded.yml up -d --pull always
```

The Compose stack publishes HTTP at `127.0.0.1:1325` and NATS at
`127.0.0.1:4223`. PostgreSQL stays on the Compose network. Override
`SVAROG_HTTP_BIND`, `SVAROG_HTTP_PORT`, `SVAROG_NATS_BIND` and `SVAROG_NATS_PORT`
to change the host bindings. Compose advertises `localhost` and the published
NATS port by default. For remote clients, set `SVAROG_EMBEDDED_NATS_PUBLIC_ADDR`
to their reachable `host:port`. Set `SVAROG_VERSION` to pin the image tag.

Run exactly one embedded replica. A second broker that opens the same
`NATS_STORE_DIR` fails to start. Use the external-NATS image with
[docker-compose.lb.yml](docker-compose.lb.yml) so committed logs reach live views
across replicas. JetStream data lives under
`$NATS_STORE_DIR/jetstream/<account public key>/`. Changing `NATS_ACCOUNT_SEED`
abandons the old stream and invalidates previously issued connection URLs.

#### Switch from external to embedded NATS

1. Stop log producers. Restart the external-NATS Svarog server without
   `SVAROG_CONN_STRING`, so its own shutdown record is ingested rather than left
   on the old broker.
2. Wait until `nats consumer info LOGS log-processor` reports
   `Unprocessed Messages: 0` and `Outstanding Acks: 0`.
3. Stop the external-NATS Svarog server and the external NATS server, so no
   client can publish to the old broker.
4. Start the embedded image with the existing PostgreSQL database, archive
   storage and APP account seed, and an empty embedded JetStream store.
5. If the client-facing `host:port` changed, update client connection URLs.
   Resume log producers once the embedded server is healthy.

## Client

In the web UI, create a project and a client, then copy the client's connection URL:

```text
svarog://<NATS_PUBLIC_ADDR>/logs.<project-uuid>.<client-uuid>?token=<credentials>
```

The token contains NATS credentials, so treat the URL as a secret. The client
prints each line it sends to standard output; append `&debug=true` to the URL
for verbose diagnostics on stderr.

Client environment variables are listed in [Configuration](#configuration).

The client image contains a static `linux/amd64` binary at `/svarog/client`:

```Dockerfile
FROM ghcr.io/markojerkic/svarog-client:latest AS svarog

FROM alpine:3.24.3
COPY --from=svarog /svarog/client /usr/local/bin/svarog-client
COPY ./your-app /app/your-app

CMD ["sh", "-o", "pipefail", "-c", "/app/your-app 2>&1 | svarog-client"]
```

To let the client drain on `docker stop`, run the container with an init
process, as the [CLI log demo](cmd/log-demo/compose.yml) does (`init: true`,
`TINI_KILL_PROCESS_GROUP=1`).

Outside Docker, build the CLI from a checkout of this repository:

```bash
go build -o svarog-client ./cmd/client
tail -F /var/log/app.log | ./svarog-client "$SVAROG_CONN_STRING"
```

### Go library

Import `github.com/markojerkic/svarog/svaroglog` to send logs directly from a Go
application without running the CLI. It requires Go 1.27.3 or newer.

```bash
go get github.com/markojerkic/svarog/svaroglog
```

Set `SVAROG_CONN_STRING` to the client URL from the web UI and call `Setup`
once at startup to forward existing `log/slog` and `log` calls:

```go
package main

import (
    "context"
    "log"
    "log/slog"

    "github.com/markojerkic/svarog/svaroglog"
)

func main() {
    closeLogging, err := svaroglog.Setup(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    slog.Info("Order created", "order_id", 42)
    log.Print("Health check passed")

    if err := closeLogging(); err != nil {
        log.Printf("Could not flush logs: %v", err)
    }
}
```

By default, INFO and higher levels are also written to stderr. Use `WithLevel`
to set the forwarding level and `WithConsole` to change or disable local output.

By default, forwarding uses a best-effort queue of 1024 records and up to 16 MiB
of encoded data. When either limit is reached, new records are dropped from
forwarding and still reach the configured console. The publisher reports
rejection counts to stderr at most once per second in both modes and flushes
any pending count when it stops. Use
`WithDropQueueCapacity(n)` to set the slot limit or
`WithDropQueueCapacity(0)` to wait for space in a 16-slot queue.

Call `closeLogging` during shutdown to wait for JetStream acknowledgements
and check for drain errors. Acknowledgement confirms acceptance by JetStream.
Canceling the context passed to `Setup` also starts shutdown.

## Development

Requires Go 1.27.3+, Bun 1.4.3, Task and Docker. `task dev` uses the
repository's development `.env` and `nats-server.conf`; do not reuse their
credentials in production.

```bash
bun install --frozen-lockfile
task dev   # PostgreSQL, NATS and the server with hot reload on http://localhost:7331
task       # Build both server images and the client image tagged :local
```

Use `task build` for the external-NATS binary (`/tmp/svarog-server`) or
`task build CMD=server-embedded` for the embedded binary (`/tmp/svarog-server-embedded`).
Pass `OUT=/path/to/binary` to choose the output path. To run the locally built
embedded image with Compose, set `SVAROG_VERSION=local` in `.env.embedded` and
start the stack without `--pull always`.

To generate demo logs, paste client connection URLs into `.env.log-demo`:

```bash
cp .env.log-demo.example .env.log-demo   # then fill in the connection URLs
docker compose -f docker-compose.log-demo.yml up -d --build
```

The `demo-spring-boot` service simulates one order every `DEMO_INTERVAL` per
replica and forwards its logs through the Svarog Spring Boot starter. Datafaker
generates product names. Logs share an `orderId` and show successful orders,
out-of-stock rejections, and payment timeouts with one retry. If both payment
attempts time out, the order logs an error with a stack trace. The demo does
not call a real payment service. Compose runs it in Spring Boot's non-web mode
so replicas can share the host network. Use milliseconds for fractional
seconds, for example `1500ms`.

Integration examples:

- [Go `slog` and `log`](cmd/log-interceptor-demo/main.go) through the public
  [`svaroglog` package](#go-library).
- [Java Logback](java/README.md) through the [Go bridge](cmd/svarog-bridge/README.md).
- [Spring Boot logger](java/svarog-spring-boot-demo/) with automatic Logback registration.

## License

Svarog is licensed under the [Apache License 2.0](LICENSE).
