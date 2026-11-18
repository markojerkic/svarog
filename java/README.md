# Svarog Logback appender

Add `svarog-logback` to a Java 17+ application that already uses SLF4J 2 and
Logback 1.5. The appender forwards logs to Svarog over the same Go bridge used
by the [standalone command](../cmd/svarog-bridge/README.md).

## Install from Maven Central

Add this dependency to your application's `pom.xml`. No token or additional
repository is needed. Maven resolves Jackson transitively; keep your
application's Logback dependency.

```xml
<dependency>
  <groupId>dev.jerkic</groupId>
  <artifactId>svarog-logback</artifactId>
  <version>0.1.0</version>
</dependency>
```

## Install the release JAR manually

Download [`svarog-logback.jar`](https://github.com/markojerkic/svarog/releases/download/latest/svarog-logback.jar)
and add it to your runtime classpath along with `com.fasterxml.jackson.core:jackson-core:2.23.0`
and your application's `logback-classic`. The JAR bundles the Go bridge for
Linux amd64 and arm64; it does not bundle Logback or Jackson.

## Configure the appender

Set `SVAROG_CONN_STRING` to the client connection URL generated in the Svarog
UI. Keep the URL out of `logback.xml` and out of version control. Optionally set
`SVAROG_INSTANCE_ID` (defaults to the hostname).

Add the appender and a reference to it in your existing `logback.xml`:

```xml
<configuration>
  <statusListener class="ch.qos.logback.core.status.OnConsoleStatusListener"/>
  <appender name="SVAROG" class="dev.jerkic.svarog.logback.SvarogAppender"/>
  <root level="INFO">
    <appender-ref ref="SVAROG"/>
  </root>
</configuration>
```

Keep an existing console or file appender if you need a local copy. See
[`svarog-demo`'s `logback.xml`](svarog-demo/src/main/resources/logback.xml)
for a complete example with console output.

Each `SvarogAppender` starts one child process. To use an existing executable,
set `<bridgePath>/absolute/path/to/svarog-bridge</bridgePath>` on the appender
or set `SVAROG_BRIDGE_PATH`. The XML setting takes precedence over the
environment and the bundled binary. Use an override if the JVM's temporary
directory (`java.io.tmpdir`) does not allow execution.

If migrating from an older build, replace the class name `DemoAppender` with
`SvarogAppender` in `logback.xml`, and rename `helperPath` to `bridgePath` if
you used that setting.

## Delivery and shutdown

Readiness means the bridge has validated local configuration; it does not
mean NATS is reachable. Appending enqueues the encoded record without
waiting for the bridge or for space. If the queue is full (128 records or
4 MiB), the newest record is dropped and the appender reports the losses
through Logback status warnings. Records the bridge cannot parse, including
messages containing NUL and encoded lines over 4 MiB, are also dropped
without stopping the bridge.

Stop the `LoggerContext` during application shutdown. The appender stops
accepting records, drains the queue, and closes the bridge's stdin. The
bridge has up to five seconds in total to finish before the appender
terminates it. A successful logging call does not confirm broker delivery
or database persistence.

## Build from source

From the repository root, `task build:java` builds `build/java/svarog-logback.jar`
with both Linux bridges and smoke-tests the packaged appender without a broker.
This requires the repository's Go toolchain and JDK 17+; Maven is provided by
the wrapper.

A plain Maven build installs the library under `dev.jerkic:svarog-logback`
at the version in `java/pom.xml` and needs an external bridge. To run the sample:

```sh
go build -o /tmp/svarog-bridge ./cmd/svarog-bridge
./java/mvnw -f java/pom.xml clean install
SVAROG_BRIDGE_PATH=/tmp/svarog-bridge \
  ./java/mvnw -f java/svarog-demo/pom.xml exec:java
```
