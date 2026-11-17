#!/usr/bin/env bash
set -euo pipefail

output_dir="${1:-build/client}"
build_dir="$(mktemp -d)"
trap 'rm -rf "$build_dir"' EXIT

for arch in amd64 arm64; do
  mkdir -p "$build_dir/$arch"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build \
    -trimpath -ldflags='-w -s' \
    -o "$build_dir/$arch/svarog-client" ./cmd/client
  tar -C "$build_dir/$arch" \
    -czf "$build_dir/svarog-client_linux_${arch}.tar.gz" svarog-client
done

(
  cd "$build_dir"
  sha256sum svarog-client_linux_amd64.tar.gz svarog-client_linux_arm64.tar.gz > checksums.txt
)

mkdir -p "$output_dir"
cp "$build_dir"/svarog-client_linux_*.tar.gz "$build_dir/checksums.txt" "$output_dir/"
