#!/usr/bin/env bash
set -euo pipefail

output_dir="${1:-build/binaries}"
targets=("${@:2}")
if (( ${#targets[@]} == 0 )); then
  targets=(client svarog-bridge)
fi

build_dir="$(mktemp -d)"
trap 'rm -rf "$build_dir"' EXIT

for target in "${targets[@]}"; do
  case "$target" in
    client) binary=svarog-client ;;
    svarog-bridge) binary=svarog-bridge ;;
    *)
      printf 'Unknown binary target: %s\n' "$target" >&2
      exit 1
      ;;
  esac

  for arch in amd64 arm64; do
    binary_dir="$build_dir/$binary/$arch"
    mkdir -p "$binary_dir"
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build \
      -trimpath -ldflags='-w -s' \
      -o "$binary_dir/$binary" "./cmd/$target"
    tar -C "$binary_dir" \
      -czf "$build_dir/${binary}_linux_${arch}.tar.gz" "$binary"
  done
done

(
  cd "$build_dir"
  sha256sum *.tar.gz > checksums.txt
)

mkdir -p "$output_dir"
cp "$build_dir"/*.tar.gz "$build_dir/checksums.txt" "$output_dir/"
