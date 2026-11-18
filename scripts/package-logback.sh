#!/usr/bin/env bash
set -euo pipefail

output_dir="$(cd "${1:-build/binaries}" && pwd)"
resources="$(mktemp -d)"
trap 'rm -f "$resources"/{svarog-bridge-linux-amd64,svarog-bridge-linux-arm64,classpath}; rmdir "$resources"' EXIT

for arch in amd64 arm64; do
  tar -xOf "$output_dir/svarog-bridge_linux_${arch}.tar.gz" svarog-bridge \
    > "$resources/svarog-bridge-linux-$arch"
done

./java/mvnw --batch-mode --no-transfer-progress -f java/pom.xml \
  "-Dsvarog.bridge.resources=$resources" clean verify
jar=(java/svarog-logback/target/svarog-logback-*.jar)
for arch in amd64 arm64; do
  unzip -p "${jar[0]}" "dev/jerkic/svarog/logback/native/svarog-bridge-linux-$arch" \
    | cmp - "$resources/svarog-bridge-linux-$arch"
done

# Use the built JAR and consumer dependencies, not the module's test classpath.
./java/mvnw --batch-mode --no-transfer-progress -f java/svarog-logback/pom.xml \
  dependency:build-classpath -DincludeScope=compile "-Dmdep.outputFile=$resources/classpath"
SVAROG_JAVA_SMOKE_CLASSPATH="$PWD/${jar[0]}:$(< "$resources/classpath")" \
  go test ./tests/client -run '^TestPackagedJavaBridge$' -count=1

cp "${jar[0]}" "$output_dir/svarog-logback.jar"
(
  cd "$output_dir"
  sed -i '/  svarog-logback\.jar$/d' checksums.txt
  sha256sum svarog-logback.jar >> checksums.txt
)
