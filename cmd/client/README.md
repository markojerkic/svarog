# Svarog CLI binaries

Download `svarog-client` from the
[latest release](https://github.com/markojerkic/svarog/releases/latest).
Running the downloaded binary does not require Docker or Go.

## Download and run

Linux amd64:

```bash
curl -fsSL https://github.com/markojerkic/svarog/releases/latest/download/svarog-client_linux_amd64.tar.gz | tar -xz
your-app | ./svarog-client "PASTE_GENERATED_CONNECTION_URL"
```

Copy the connection URL from the Svarog web UI. For Linux arm64, replace `amd64`
with `arm64` in the download URL.

## Build locally

From the repository root, using the Go version specified in `go.mod`:

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
Builds use `CGO_ENABLED=0`, so the Linux binaries do not depend on a system C
library.

## Pin a version

To pin a version, use `/releases/download/vX.Y.Z/<filename>` instead of
`/releases/latest/download/<filename>`. Each release includes one
`checksums.txt` for the published binaries.
