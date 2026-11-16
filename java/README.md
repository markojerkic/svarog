# Java logging demo

Requires JDK 17+ and Maven.

- `svarog-logback`: a minimal Logback appender that prints received events.
- `svarog-demo`: an application that depends on the library and calls SLF4J.

From the repository root:

```sh
./mvnw -f java/pom.xml install
./mvnw -f java/svarog-demo/pom.xml exec:java
```

Expected application output:

```text
[library] INFO Hello from svarog-demo!
```

Flow: SLF4J → Logback → `DemoAppender` in the library.
There is no Svarog transport or Go helper integration yet.
