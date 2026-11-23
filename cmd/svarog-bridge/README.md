# Svarog bridge

`svarog-bridge` reads JSON log entries from stdin and forwards them to Svarog.
Unlike the [CLI client](../client/README.md), which timestamps plain-text lines
when it reads them, the bridge keeps each entry's own timestamp.

The [Java appender](../../java/README.md) bundles the bridge and starts it
automatically. Run it yourself only to integrate another runtime, or to replace
the bundled executable through `SVAROG_BRIDGE_PATH`.

## Download and run

Linux amd64:

```bash
curl -fsSL https://github.com/markojerkic/svarog/releases/latest/download/svarog-bridge_linux_amd64.tar.gz | tar -xz
export SVAROG_CONN_STRING='PASTE_GENERATED_CONNECTION_URL'
printf '{"message":"Hello from the bridge","timestamp":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" | ./svarog-bridge
```

Copy the connection URL from the Svarog web UI. For Linux arm64, replace
`amd64` with `arm64` in the download URL. To pin a version, use
`/releases/download/vX.Y.Z/<filename>` instead of
`/releases/latest/download/<filename>`.

## Configuration and protocol

| Setting              | Default  | Purpose                             |
| -------------------- | -------- | ----------------------------------- |
| `SVAROG_CONN_STRING` | Required | Connection URL.                     |
| `SVAROG_INSTANCE_ID` | Hostname | Instance name attached to the logs. |

1. After it reads its configuration, the bridge writes `{"v":1,"op":"ready"}`
   to stdout. It connects to NATS afterwards.
2. Write one JSON object per line to its stdin, with a `message` and an
   RFC 3339 `timestamp`. A line may be up to 4 MiB.
3. Close stdin to stop it. The bridge waits up to 4 seconds for JetStream to
   acknowledge accepted entries.

The bridge exits with status 1 and an error on stderr when a line is not valid
JSON, has no timestamp or cannot be queued, or when the drain fails.

## Build locally

```bash
task build:bridge
```

This creates `svarog-bridge_linux_amd64.tar.gz`,
`svarog-bridge_linux_arm64.tar.gz` and `checksums.txt` under `build/bridge`.
Use `OUT=/path/to/output` to select another directory.

`task build:binaries` builds both the client and the bridge under
`build/binaries`, with one shared `checksums.txt`.
