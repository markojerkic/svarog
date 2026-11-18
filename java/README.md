# Svarog logging for Java

Requires Java 17+, SLF4J 2 and Logback 1.5. Set `SVAROG_CONN_STRING` to the
connection URL from the Svarog UI. Keep it out of version control.

The bridge is bundled for Linux amd64 and arm64. To use a different
executable, set `SVAROG_BRIDGE_PATH` to its absolute path.

## Spring Boot 3.5

Add the starter to `pom.xml`, replacing `RELEASE_VERSION` with its version on
Maven Central once published:

```xml
<dependency>
  <groupId>dev.jerkic</groupId>
  <artifactId>svarog-spring-boot-starter</artifactId>
  <version>RELEASE_VERSION</version>
</dependency>
```

Set `svarog.logging.enabled=true` in `application.properties` or
`SVAROG_LOGGING_ENABLED=true` in the environment. No `logback-spring.xml`
is needed. See the [Spring Boot demo](svarog-spring-boot-demo).

## Plain Logback

Add the appender from Maven Central:

```xml
<dependency>
  <groupId>dev.jerkic</groupId>
  <artifactId>svarog-logback</artifactId>
  <version>0.0.1</version>
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

Stop the `LoggerContext` on shutdown to drain queued logs. See the
[Logback demo](svarog-demo/src/main/resources/logback.xml) for a console example.
