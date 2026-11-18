# Svarog bridge

The bridge reads JSON log entries from stdin and forwards them to Svarog. Each
entry has a `message` and an RFC 3339 `timestamp`. For plain-text logs, use the
[CLI client](../client/README.md) instead.

## Download and run

Linux amd64:

```bash
curl -fsSL https://github.com/markojerkic/svarog/releases/latest/download/svarog-bridge_linux_amd64.tar.gz | tar -xz
export SVAROG_CONN_STRING='PASTE_GENERATED_CONNECTION_URL'
printf '{"message":"Hello from Java","timestamp":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" | ./svarog-bridge
```

Copy the connection URL from the Svarog web UI. For Linux arm64, replace `amd64`
with `arm64` in the download URL. `SVAROG_INSTANCE_ID` optionally overrides the
hostname.

The bridge writes `{"v":1,"op":"ready"}` to stdout before reading entries.
The published [Logback JAR](../../java/README.md) already bundles the bridge.
In Java applications, configure `dev.jerkic.svarog.logback.SvarogAppender`.
Set `SVAROG_BRIDGE_PATH` to the downloaded executable's absolute path if you
want to override the bundled bridge or are using a JAR built without one.

## Build locally

```bash
task build:bridge
```

This creates `svarog-bridge_linux_amd64.tar.gz`,
`svarog-bridge_linux_arm64.tar.gz` and `checksums.txt` under `build/bridge`.
Use `OUT=/path/to/output` to select another directory.

`task build:binaries` builds both the client and bridge under `build/binaries`,
with one shared `checksums.txt`. These archives are available from the
[latest release](https://github.com/markojerkic/svarog/releases/latest) or a
specific `vX.Y.Z` release.
