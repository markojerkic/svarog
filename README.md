# Svarog

Svarog collects application logs over NATS JetStream, stores them in PostgreSQL
and shows them in a web UI.

- Projects group log-producing clients. Each client sends logs from one or more
  named instances, such as hosts.
- Live tailing, substring search, date ranges and context around a log line.
- Scheduled and manual ZIP archiving per client, on disk or in S3-compatible
  storage. Administrators can search archived logs.
- User accounts with per-project access.
- An optional [MCP endpoint](#mcp) for AI agents.

Applications send logs through the [CLI](cmd/client/README.md), the
[Go library](svaroglog/README.md) or the [Java appender](java/README.md). Each
of them publishes to NATS JetStream. The server stores the logs in PostgreSQL
and streams new lines to open log views.

## Contents

- [Deploy the server](#deploy-the-server)
  - [Choose an image](#choose-an-image)
  - [Generate NATS credentials](#generate-nats-credentials)
  - [Deploy with external NATS](#deploy-with-external-nats)
  - [Deploy with embedded NATS](#deploy-with-embedded-nats)
  - [Compose variables](#compose-variables)
  - [First login and reverse proxy](#first-login-and-reverse-proxy)
  - [Switch from external to embedded NATS](#switch-from-external-to-embedded-nats)
- [Send logs](#send-logs)
- [MCP](#mcp)
- [Configuration](#configuration)
  - [Required settings](#required-settings)
  - [Common settings](#common-settings)
  - [Archive storage](#archive-storage)
  - [Server self-logging](#server-self-logging)
  - [Embedded NATS listener](#embedded-nats-listener)
  - [Ingestion tuning](#ingestion-tuning)
  - [Other settings](#other-settings)
- [Development](#development)
- [License](#license)

## Deploy the server

### Choose an image

Both images are published with `vX.Y.Z` and `latest` tags and require
PostgreSQL.

|                 | External NATS                                                          | Embedded NATS                                                               |
| --------------- | ---------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| Image           | `ghcr.io/markojerkic/svarog`                                           | `ghcr.io/markojerkic/svarog-embedded`                                       |
| NATS broker     | Separate authenticated NATS server                                     | Runs inside the server process                                              |
| Replicas        | One or more                                                            | Exactly one                                                                 |
| Compose example | [docker-compose.prod.yml](docker-compose.prod.yml), without PostgreSQL | [docker-compose.embedded.yml](docker-compose.embedded.yml), with PostgreSQL |

For several replicas behind a load balancer, see
[docker-compose.lb.yml](docker-compose.lb.yml). It is a development example
with three external-NATS replicas behind nginx. Committed logs reach live views
on every replica through the shared NATS server.

### Generate NATS credentials

Both images need a NATS account seed. From a checkout of this repository, run:

```bash
go run ./cmd/nats-setup
```

It prints values for `.env` and a configuration for `nats-server.conf`. The
embedded image needs only `NATS_ACCOUNT_SEED` from that output.

### Deploy with external NATS

[docker-compose.prod.yml](docker-compose.prod.yml) runs the server and an
authenticated NATS server. PostgreSQL is not included and must already exist.

1. Save the configuration printed by `nats-setup` as
   `$SVAROG_DEPLOY_DIR/nats-server.conf` and append JetStream storage:

   ```conf
   jetstream {
     store_dir: /data/jetstream
   }
   ```

2. Create `.env`:

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
   # Optional: enables the MCP endpoint
   # PUBLIC_URL=https://logs.example.com
   ```

3. Start the stack:

   ```bash
   docker compose --env-file .env -f docker-compose.prod.yml up -d --pull always
   ```

The web UI is published on `127.0.0.1:1323` and NATS on `0.0.0.0:4222`. See
[Compose variables](#compose-variables) to change them.

### Deploy with embedded NATS

[docker-compose.embedded.yml](docker-compose.embedded.yml) runs the embedded
image with its own PostgreSQL and volumes for JetStream and disk archives.

1. Create `.env.embedded`:

   ```dotenv
   SESSION_SECRET=replace-with-a-long-random-secret
   # Printed by nats-setup
   NATS_ACCOUNT_SEED=...
   # Must be URL-safe
   POSTGRES_PASSWORD=replace-with-a-database-password
   # Client-facing host:port; defaults to localhost and the published NATS port
   # SVAROG_EMBEDDED_NATS_PUBLIC_ADDR=logs.example.com:4223
   ```

2. Start the stack:

   ```bash
   docker compose --env-file .env.embedded -f docker-compose.embedded.yml up -d --pull always
   ```

The web UI is published on `127.0.0.1:1325` and NATS on `127.0.0.1:4223`.
PostgreSQL stays on the Compose network. For remote clients, set
`SVAROG_EMBEDDED_NATS_PUBLIC_ADDR` to their reachable `host:port`.

Limits and security notes:

- Run exactly one replica. A second broker that opens the same `NATS_STORE_DIR`
  fails to start. For several replicas, use the external-NATS image.
- The NATS listener has no TLS option. Log contents travel in plaintext, so keep
  the listener on a private network or behind an encrypted tunnel.
- The broker accepts any user JWT signed by the APP account key. That includes
  the `NATS_SERVER_USER_JWT` that `nats-setup` creates, which may publish and
  subscribe to every subject and never expires. Keep it secret after switching
  to the embedded image.
- Revocations and signing keys of a customized external account are not carried
  over. For such an account, use a new `NATS_ACCOUNT_SEED` and reissue client
  connection URLs.
- JetStream data lives under `$NATS_STORE_DIR/jetstream/<account public key>/`.
  Changing `NATS_ACCOUNT_SEED` abandons the old stream and invalidates
  previously issued connection URLs.

The image generates the NATS operator and `$SYS` account on every start. It
does not use `NATS_ADDR`, `NATS_SERVER_USER_JWT`, `NATS_SERVER_USER_SEED` or
`nats-server.conf`. The broker exposes only the NATS client listener, so use
HTTP `/login` for health checks.

### Compose variables

Docker Compose reads these variables from the `--env-file`. They configure the
Compose files, not the server itself.

| Variable                           | `docker-compose.prod.yml` | `docker-compose.embedded.yml` | Purpose                                                                 |
| ---------------------------------- | ------------------------- | ----------------------------- | ----------------------------------------------------------------------- |
| `SVAROG_IMAGE`                     | Required                  | Not used                      | Server image. Pin a `vX.Y.Z` tag instead of `latest`.                   |
| `SVAROG_DEPLOY_DIR`                | Required                  | Not used                      | Absolute host directory containing `nats-server.conf`.                  |
| `POSTGRES_PASSWORD`                | Not used                  | Required                      | URL-safe password for the bundled PostgreSQL.                           |
| `SVAROG_VERSION`                   | Not used                  | `latest`                      | Embedded image tag. Pin a `vX.Y.Z` tag instead of `latest`.             |
| `SVAROG_EMBEDDED_NATS_PUBLIC_ADDR` | Not used                  | `localhost:$SVAROG_NATS_PORT` | Client-facing NATS address, passed to the server as `NATS_PUBLIC_ADDR`. |
| `SVAROG_HTTP_BIND`                 | `127.0.0.1`               | `127.0.0.1`                   | Host address on which the web UI is published.                          |
| `SVAROG_HTTP_PORT`                 | `1323`                    | `1325`                        | Host port for the web UI.                                               |
| `SVAROG_NATS_BIND`                 | `0.0.0.0`                 | `127.0.0.1`                   | Host address on which NATS is published.                                |
| `SVAROG_NATS_PORT`                 | `4222`                    | `4223`                        | Host port for NATS clients.                                             |

Both Compose files set the [required settings](#required-settings) and forward
`PUBLIC_URL`, `LOG_LEVEL`, `ARCHIVE_SCHEDULER_ENABLED` and the
[self-logging](#server-self-logging) variables to the server. Disk archives are
stored in a named volume. To use any other [server setting](#configuration),
such as S3 archive storage, add it to the `svarog` service's `environment`.

### First login and reverse proxy

The server runs database migrations on startup. A fresh database gets the user
`admin` / `ADMINADMIN`, who must change the password at first login.

A reverse proxy in front of the web UI must forward WebSocket upgrades, which
live log views use. When it terminates TLS, it must also send
`X-Forwarded-Proto: https` so session cookies are marked secure.

### Switch from external to embedded NATS

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

## Send logs

In the web UI, create a project and a client, then copy the client's connection
URL:

```text
svarog://<NATS_PUBLIC_ADDR>/logs.<project-uuid>.<client-uuid>?token=<credentials>
```

The token contains NATS credentials, so treat the URL as a secret. Append
`&debug=true` for verbose client diagnostics on stderr.

| Integration                                                | Use it for                                                         |
| ---------------------------------------------------------- | ------------------------------------------------------------------ |
| [`svarog-client` CLI](cmd/client/README.md)                | Any program whose output can be piped, including Docker containers |
| [Go library `svaroglog`](svaroglog/README.md)              | Go applications that log through `log/slog` or `log`               |
| [Spring Boot starter and Logback appender](java/README.md) | Java 17+ applications that use SLF4J and Logback                   |
| [`svarog-bridge`](cmd/svarog-bridge/README.md)             | Other runtimes that can write JSON lines to a child process        |

Every integration reads the connection URL from `SVAROG_CONN_STRING` unless it
is configured another way. Logs are attached to the instance named by
`SVAROG_INSTANCE_ID`, or to the hostname when it is unset.

## MCP

With `PUBLIC_URL=https://logs.example.com`, AI agents can connect to the MCP
endpoint `https://logs.example.com/mcp`, for example with
`claude mcp add --transport http svarog https://logs.example.com/mcp`. Clients
must use exactly this URL, because OAuth tokens are bound to it.

The MCP client opens a Svarog page in the browser, where the user signs in and
allows access. The client then receives an access token valid for one hour and
a refresh token valid for 30 days since its last use. It sees the same projects
as its user in the web UI. Changing the user's password or generating a new
login link disconnects all of the user's MCP clients.

Clients identify themselves with an OAuth Client ID Metadata Document, as the
MCP specification recommends. Clients that support only Dynamic Client
Registration cannot connect. Svarog fetches client metadata only over HTTPS and
only from public IP addresses.

## Configuration

The server reads its settings from environment variables. Unless `DOCKER=true`,
as in the published images, it also loads `.env.local` and then `.env` from the
working directory. The process environment takes precedence over `.env.local`,
which takes precedence over `.env`.

Settings are grouped from most to least commonly needed. Unless noted
otherwise, a setting applies to both images.

### Required settings

| Variable                | Purpose                                                                                                                                                                                                                                                    |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POSTGRES_URL`          | Direct PostgreSQL connection URL; a transaction-pooling proxy is not supported. The database user must be able to create the `pg_trgm` extension unless it is already installed. When unset, the server uses the development database on `localhost:5434`. |
| `SESSION_SECRET`        | Long random secret that signs session cookies and archive download links.                                                                                                                                                                                  |
| `NATS_PUBLIC_ADDR`      | Client-facing `host:port` written into connection URLs, e.g. `logs.example.com:4222`.                                                                                                                                                                      |
| `NATS_ACCOUNT_SEED`     | Account seed from [`nats-setup`](#generate-nats-credentials). Signs client credentials.                                                                                                                                                                    |
| `NATS_ADDR`             | External NATS only. Broker URL the server connects to. `docker-compose.prod.yml` sets `nats://nats:4222`.                                                                                                                                                  |
| `NATS_SERVER_USER_JWT`  | External NATS only. Server user JWT from `nats-setup`.                                                                                                                                                                                                     |
| `NATS_SERVER_USER_SEED` | External NATS only. Seed paired with `NATS_SERVER_USER_JWT`.                                                                                                                                                                                               |

### Common settings

| Variable     | Default | Purpose                                                                                                                                          |
| ------------ | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `PUBLIC_URL` | Unset   | Public origin of the web UI, e.g. `https://logs.example.com`. Enables the [MCP endpoint](#mcp) at `/mcp`. Must use HTTPS unless it is localhost. |
| `LOG_LEVEL`  | `info`  | Minimum severity of server logs: `debug`, `info`, `warn` or `error`.                                                                             |

### Archive storage

Svarog stores archives in S3-compatible storage when any of `ARCHIVE_S3_BUCKET`,
`ARCHIVE_S3_ENDPOINT`, `ARCHIVE_S3_ACCESS_KEY_ID` or
`ARCHIVE_S3_SECRET_ACCESS_KEY` is set; all four are then required. Otherwise it
stores them in `ARCHIVE_DIR`. Without either, archiving is disabled.

| Variable                       | Default                                        | Purpose                                                                    |
| ------------------------------ | ---------------------------------------------- | -------------------------------------------------------------------------- |
| `ARCHIVE_DIR`                  | Unset; Compose sets `/var/lib/svarog/archives` | Directory for disk archives.                                               |
| `ARCHIVE_SCHEDULER_ENABLED`    | `true`                                         | Run archive policies automatically. Manual archiving works in either case. |
| `ARCHIVE_S3_BUCKET`            | Unset                                          | Existing destination bucket.                                               |
| `ARCHIVE_S3_ENDPOINT`          | Unset                                          | S3-compatible endpoint URL. Requests use path-style addressing.            |
| `ARCHIVE_S3_ACCESS_KEY_ID`     | Unset                                          | Access key ID.                                                             |
| `ARCHIVE_S3_SECRET_ACCESS_KEY` | Unset                                          | Secret access key.                                                         |
| `ARCHIVE_S3_REGION`            | `auto`                                         | Region used for S3 requests.                                               |

Archive search returns up to 200 matches per page, starting with the newest.
Older pages appear above the existing rows, as in the ordinary log viewer. Each
request scans at most 128 ZIPs, 256 MiB compressed, 512 MiB expanded and 8 MiB
of result text, with a one-minute timeout. If a limit is exceeded or a ZIP
cannot be verified, no partial results are shown.

### Server self-logging

The server can forward its own logs to a client created in the web UI.

| Variable                    | Default  | Purpose                                                                                                                                                                                    |
| --------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `SVAROG_CONN_STRING`        | Unset    | Client connection URL for the server's own logs. Unset means console only.                                                                                                                 |
| `SVAROG_INSTANCE_ID`        | Hostname | Instance name attached to the server's logs.                                                                                                                                               |
| `SVAROG_LOG_QUEUE_CAPACITY` | `1024`   | Records waiting to be forwarded, from 1 to 65536; invalid values use the default. The queue also caps encoded data at 16 MiB. A record that exceeds either limit goes only to the console. |

In embedded mode, the server connects to its own broker, so the URL's
`host:port` must be reachable from inside the container. When the published
port differs from `NATS_LISTEN_PORT`, as in `docker-compose.embedded.yml`,
replace the copied URL's `host:port` with `localhost:4222`.

### Embedded NATS listener

These settings apply only to the embedded image.

| Variable           | Default                | Purpose                                 |
| ------------------ | ---------------------- | --------------------------------------- |
| `NATS_LISTEN_HOST` | `0.0.0.0`              | Address the embedded broker listens on. |
| `NATS_LISTEN_PORT` | `4222`                 | Port the embedded broker listens on.    |
| `NATS_STORE_DIR`   | `/var/lib/svarog/nats` | Persistent JetStream storage directory. |

### Ingestion tuning

| Variable                           | Default | Purpose                                                                                                                                    |
| ---------------------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `INGEST_STREAM_MAX_BYTES`          | `0`     | Size limit of the JetStream `LOGS` stream in bytes. `0` keeps the existing limit or sets 1 GiB for a new stream.                           |
| `INGEST_ACK_WAIT`                  | `60s`   | How long JetStream waits for an acknowledgement before a delivery is eligible for retry.                                                   |
| `INGEST_ALLOW_RETENTION_MIGRATION` | `false` | Allow migrating a legacy log stream's retention policy. Back up the stream and consumer state and stop all old writers before enabling it. |

### Other settings

| Variable                  | Default              | Purpose                                                            |
| ------------------------- | -------------------- | ------------------------------------------------------------------ |
| `HTTP_SERVER_PORT`        | `1323` in the images | Port the web UI listens on.                                        |
| `DOCKER`                  | `true` in the images | Skip loading `.env.local` and `.env`.                              |
| `NATS_ACCOUNT_PUBLIC_KEY` | Unset                | Printed by `nats-setup` for reference. The server does not use it. |
| `SERVER_DNS_NAME`         | Unset                | Parsed but currently unused.                                       |

## Development

Requires Go 1.27.3+, Bun 1.4.3, Task and Docker.

```bash
bun install --frozen-lockfile
task dev           # PostgreSQL in Docker, embedded NATS, hot reload at http://localhost:7331
task dev:external  # Same hot reload, with PostgreSQL and NATS in Docker
```

Both modes use the development `.env`; put local overrides, such as S3
credentials or a self-logging URL, in the untracked `.env.local`. The external
broker also uses `nats-server.conf`. Do not reuse these credentials in
production.

The development PostgreSQL uses `svarog` as its database, user and password.
Both brokers listen on `localhost:4222`. `task dev` stops the development NATS
container to free that port and stores embedded JetStream data in `tmp/nats`.

Build commands:

| Command                          | Output                                                            |
| -------------------------------- | ----------------------------------------------------------------- |
| `task`                           | Server, embedded server and client Docker images, tagged `:local` |
| `task build`                     | External-NATS server binary at `/tmp/svarog-server`               |
| `task build CMD=server-embedded` | Embedded server binary at `/tmp/svarog-server-embedded`           |
| `task build:binaries`            | CLI and bridge archives with checksums under `build/binaries`     |
| `task build:java`                | Logback JAR with bundled bridges under `build/java`               |

Pass `OUT=/path` to choose another output path. To run the locally built
embedded image with Compose, set `SVAROG_VERSION=local` in `.env.embedded` and
start the stack without `--pull always`.

Tests:

```bash
go tool templ generate ./...                # Generated templates are not tracked
go test ./tests/...                         # Starts PostgreSQL and NATS with Testcontainers
bun run playwright:setup && bun run test    # Browser tests
```

Development-only variables:

| Variable                             | Default | Purpose                                                                                              |
| ------------------------------------ | ------- | ---------------------------------------------------------------------------------------------------- |
| `SVAROG_POSTGRES_PORT`               | `5434`  | Host port of the development PostgreSQL container.                                                   |
| `PROXY_PORT`                         | `7331`  | Port of the hot-reload proxy.                                                                        |
| `SHADCN_TEMPL_DEV`, `TEMPL_DEV_MODE` | Unset   | Either set to `true` disables browser caching of UI assets. `task dev` sets `SHADCN_TEMPL_DEV=true`. |
| `GO_ENV`                             | Unset   | `development` is the legacy alias for disabling asset caching.                                       |
| `SVAROG_VERSION`                     | `local` | Server image tag for `docker-compose.yml` and `docker-compose.lb.yml`.                               |
| `GPRC_PORT`                          | Unused  | Legacy entry in the development `.env`; it does not configure a listener.                            |

To fill the UI with generated logs from several clients, see the
[log demo](cmd/log-demo/README.md).

## License

Svarog is licensed under the [Apache License 2.0](LICENSE).
