# Svarog

Svarog collects application logs over NATS, stores them in PostgreSQL and shows
them in a web UI.

- Projects with clients, each sending logs from one or more named instances.
- Live tailing, substring search, date ranges and context around a log line.
- Scheduled and manual ZIP archiving per client, on disk or in S3-compatible storage.
- User accounts with per-project access.

Applications pipe their output into the Svarog client, which publishes it to
NATS JetStream; the server stores it in PostgreSQL and streams new lines to the UI.

## Server

[docker-compose.prod.yml](docker-compose.prod.yml) runs the server and an
authenticated NATS server. PostgreSQL is not included and must already exist.
`POSTGRES_URL` must be a direct connection, not a transaction-pooling proxy, and
its user must be able to create the `pg_trgm` extension unless it already exists.

### NATS credentials

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
# Absolute host directory containing nats-server.conf
SVAROG_DEPLOY_DIR=/absolute/path/to/svarog
# host.docker.internal is the Docker host; PostgreSQL must accept connections from containers
POSTGRES_URL=postgres://svarog:password@host.docker.internal:5432/svarog
SESSION_SECRET=replace-with-a-long-random-secret
# Host and port clients use to reach NATS; written into connection URLs
NATS_PUBLIC_ADDR=logs.example.com:4222
# Printed by nats-setup
NATS_ACCOUNT_SEED=...
NATS_SERVER_USER_JWT=...
NATS_SERVER_USER_SEED=...
```

Optional settings:

| Variable                    | Default     | Purpose                            |
| --------------------------- | ----------- | ---------------------------------- |
| `SVAROG_HTTP_BIND`          | `127.0.0.1` | Host address of the web UI         |
| `SVAROG_HTTP_PORT`          | `1323`      | Host port of the web UI            |
| `SVAROG_NATS_BIND`          | `0.0.0.0`   | Host address for NATS clients      |
| `SVAROG_NATS_PORT`          | `4222`      | Host port for NATS clients         |
| `LOG_LEVEL`                 | `info`      | Server log level                   |
| `ARCHIVE_SCHEDULER_ENABLED` | `true`      | Run archive policies automatically |

Other [ServerEnv](internal/server/types/env.go) settings, such as `ARCHIVE_S3_*`
for S3 archive storage, must be added explicitly to the `svarog` service's
`environment`; values in `.env` alone do not reach the container.

Start the server:

```bash
docker compose --env-file .env -f docker-compose.prod.yml up -d --pull always
```

Migrations run on startup. A fresh database gets the user `admin` /
`ADMINADMIN`, who must change the password at first login.

The web UI listens on `127.0.0.1` by default. A reverse proxy in front of it
must forward WebSocket upgrades for live logs and, when it terminates TLS, send
`X-Forwarded-Proto: https` so session cookies are marked secure.

## Client

In the web UI, create a project and a client, then copy the client's connection URL:

```text
svarog://<NATS_PUBLIC_ADDR>/logs.<project-uuid>.<client-uuid>?token=<credentials>
```

The token contains NATS credentials, so treat the URL as a secret. The client
prints each line it sends to standard output; append `&debug=true` to the URL
for verbose diagnostics on stderr.

| Variable             | Description                                                               |
| -------------------- | ------------------------------------------------------------------------- |
| `SVAROG_CONN_STRING` | Connection URL, unless it is passed as the first argument                 |
| `SVAROG_INSTANCE_ID` | Optional instance name; defaults to the hostname (container ID in Docker) |

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

## Development

Requires Go 1.27.3+, Bun 1.4.3, Task and Docker. `task dev` uses the
repository's development `.env` and `nats-server.conf`; do not reuse their
credentials in production.

```bash
bun install --frozen-lockfile
task dev   # PostgreSQL, NATS and the server with hot reload on http://localhost:7331
task       # Build server and client images tagged :local
```

To generate demo logs, paste client connection URLs into `.env.log-demo`:

```bash
cp .env.log-demo.example .env.log-demo   # then fill in the connection URLs
docker compose -f docker-compose.log-demo.yml up -d --build
```

Integration examples:

- [Go `slog` and `log`](cmd/log-interceptor-demo/main.go) through the internal
  `svaroglog` package, which other Go modules cannot import.
- [Java Logback](java/README.md) through the [Go bridge](cmd/svarog-bridge/README.md).

## License

Svarog is licensed under the [Apache License 2.0](LICENSE).
