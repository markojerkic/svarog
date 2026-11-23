# Svarog logging for Java

Forward SLF4J and Logback logs from a Java application to Svarog. Requires
Java 17+, SLF4J 2 and Logback 1.5.

The appender starts the [Svarog bridge](../cmd/svarog-bridge/README.md) as a
child process and sends it each log event. The JAR bundles the bridge for Linux
amd64 and arm64.

Both artifacts are published to
[Maven Central](https://central.sonatype.com/namespace/dev.jerkic) under
`dev.jerkic`, with the same version as the Svarog release. The examples below
use `0.1.3`.

- [Spring Boot](#spring-boot)
- [Plain Logback](#plain-logback)
- [Configuration](#configuration)
- [Delivery and shutdown](#delivery-and-shutdown)
- [Build locally](#build-locally)

## Spring Boot

The starter supports Spring Boot 3.5. Add it to `pom.xml`:

```xml
<dependency>
  <groupId>dev.jerkic</groupId>
  <artifactId>svarog-spring-boot-starter</artifactId>
  <version>0.1.3</version>
</dependency>
```

Set `SVAROG_CONN_STRING` and enable forwarding with
`svarog.logging.enabled=true` in `application.properties` or
`SVAROG_LOGGING_ENABLED=true` in the environment.

The starter adds the appender to the root logger, next to any existing
appenders, so no `logback-spring.xml` is needed. It stops the appender when the
application context closes. See the [Spring Boot demo](svarog-spring-boot-demo).

## Plain Logback

Add the appender to `pom.xml`:

```xml
<dependency>
  <groupId>dev.jerkic</groupId>
  <artifactId>svarog-logback</artifactId>
  <version>0.1.3</version>
</dependency>
```

Register it in `logback.xml` and keep any existing appender references:

```xml
<configuration>
  <appender name="SVAROG" class="dev.jerkic.svarog.logback.SvarogAppender"/>
  <root level="INFO">
    <appender-ref ref="SVAROG"/>
  </root>
</configuration>
```

Set `SVAROG_CONN_STRING`, and stop the `LoggerContext` on shutdown so queued
logs are drained:

```java
var context = (LoggerContext) LoggerFactory.getILoggerFactory();
context.stop();
```

The [Logback demo](svarog-demo) also logs to the console and prints Logback
status messages, which include the appender's warnings and errors.

## Configuration

| Setting                                              | Default        | Purpose                                                                                                                   |
| ---------------------------------------------------- | -------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `SVAROG_CONN_STRING`                                 | Required       | Connection URL from the Svarog web UI. Keep it out of version control.                                                    |
| `svarog.logging.enabled` or `SVAROG_LOGGING_ENABLED` | `false`        | Spring Boot only. Enables forwarding.                                                                                     |
| `SVAROG_INSTANCE_ID`                                 | Hostname       | Instance name attached to the logs.                                                                                       |
| `SVAROG_BRIDGE_PATH`                                 | Bundled bridge | Absolute path to another bridge executable, for example on a platform without a bundled bridge.                           |
| `svarog.logging.bridge-path` or `<bridgePath>`       | Unset          | Bridge path set in Spring Boot properties or inside the Logback `<appender>`. Takes precedence over `SVAROG_BRIDGE_PATH`. |

Release archives contain the bridge for Linux only. For another platform, build
it from this repository with `go build ./cmd/svarog-bridge` and point
`SVAROG_BRIDGE_PATH` to the result.

## Delivery and shutdown

- Each event is sent as `LEVEL logger - message`, followed by the stack trace
  when there is one. The original event timestamp is preserved.
- Forwarding is best effort. The appender queues up to 128 records and 4 MiB.
  When the queue is full, it drops new records. Records larger than 4 MiB are
  also dropped. Drop counts are reported as Logback status warnings at most
  once every 10 seconds.
- If the bridge does not report readiness within 5 seconds, the appender logs
  an error and stays stopped.
- On stop, the appender waits up to 5 seconds for the bridge to drain and
  reports an error if delivery is not confirmed.

## Build locally

From the repository root:

```bash
task build:java
```

The task builds the Linux bridges, packages them into
`build/java/svarog-logback.jar`, runs a smoke test against the packaged JAR and
adds the JAR to `build/java/checksums.txt`. Use `OUT=/path/to/output` to select
another directory.
