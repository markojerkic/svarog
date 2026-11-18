#!/usr/bin/env bash
set -euo pipefail

output_dir="$(cd "${1:-build/binaries}" && pwd)"
maven_args=("${@:2}")
if (( ${#maven_args[@]} == 0 )); then
  maven_args=(verify)
fi
resources="$(mktemp -d)"
trap 'rm -rf -- "$resources"' EXIT

for arch in amd64 arm64; do
  tar -xOf "$output_dir/svarog-bridge_linux_${arch}.tar.gz" svarog-bridge \
    > "$resources/svarog-bridge-linux-$arch"
done

# The bridge is statically linked, so ship its Go dependencies' license text.
modules="$(
  for arch in amd64 arm64; do
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go list -deps \
      -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Dir}}{{end}}{{end}}' \
      ./cmd/svarog-bridge || exit
  done | sort -u
)"
go_license_dir="$(go env GOROOT)"
if [[ ! -f "$go_license_dir/LICENSE" ]]; then
  go_license_dir=/usr/share/licenses/go
fi
while read -r module dir; do
  [[ -n "$module" ]] || continue
  mkdir -p "$resources/licenses/$module"
  cp "$dir/LICENSE" "$resources/licenses/$module/"
  for notice in NOTICE PATENTS; do
    if [[ -f "$dir/$notice" ]]; then
      cp "$dir/$notice" "$resources/licenses/$module/"
    fi
  done
done <<< "$modules"$'\n'"go $go_license_dir"

./java/mvnw --batch-mode --no-transfer-progress -f java/pom.xml \
  "-Dsvarog.bridge.resources=$resources" clean "${maven_args[@]}"
jar=java/svarog-logback/target/svarog-logback.jar
for arch in amd64 arm64; do
  unzip -p "$jar" "dev/jerkic/svarog/logback/native/svarog-bridge-linux-$arch" \
    | cmp - "$resources/svarog-bridge-linux-$arch"
done

# Use the built JAR and consumer dependencies, not the module's test classpath.
./java/mvnw --batch-mode --no-transfer-progress -f java/svarog-logback/pom.xml \
  dependency:build-classpath -DincludeScope=compile "-Dmdep.outputFile=$resources/classpath"
SVAROG_JAVA_SMOKE_CLASSPATH="$PWD/$jar:$(< "$resources/classpath")" \
  go test ./tests/client -run '^TestPackagedJavaBridge$' -count=1

cp "$jar" "$output_dir/svarog-logback.jar"
(
  cd "$output_dir"
  sed -i '/  svarog-logback\.jar$/d' checksums.txt
  sha256sum svarog-logback.jar >> checksums.txt
)
