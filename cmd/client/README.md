# Svarog CLI binaries

Download `svarog-client` from the
[latest release](https://github.com/markojerkic/svarog/releases/tag/latest).
Running the downloaded binary does not require Docker or Go.

## Download and run

Linux amd64:

```bash
curl -fsSL https://github.com/markojerkic/svarog/releases/download/latest/svarog-client_linux_amd64.tar.gz | tar -xz
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

## Publication

`.github/workflows/publish-binaries.yml` runs on pushes to `master` and can also be
started manually for `master`. Client and bridge archives share one GitHub
Release tagged `latest` and one `checksums.txt`. Later successful builds replace
the assets and move the tag to the build's commit. Existing historical releases
are not deleted.

Release updates are serialized so two builds cannot replace assets at the same
time. Authentication uses the built-in `GITHUB_TOKEN` with `contents: write`;
there are no additional secrets. The `latest` tag must be allowed to move, and
GitHub release immutability must remain disabled for this rolling release.
