package dev.jerkic.svarog.springboot.autoconfigure;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.concurrent.TimeUnit;

import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.LoggerContext;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.Appender;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledOnOs;
import org.junit.jupiter.api.condition.OS;
import org.junit.jupiter.api.io.TempDir;
import org.slf4j.LoggerFactory;
import org.springframework.boot.WebApplicationType;
import org.springframework.boot.autoconfigure.EnableAutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.boot.builder.SpringApplicationBuilder;
import org.springframework.context.ConfigurableApplicationContext;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.StandardEnvironment;

public class SvarogLoggingAutoConfigurationTest {
    @TempDir
    public Path directory;

    @Test
    public void leavesDefaultLoggingAloneWhenNotEnabled() {
        try (var context = run("--svarog.logging.bridge-path=" + this.directory.resolve("missing"))) {
            assertFalse(context.containsBean("svarogAppenderRegistration"));
            assertNull(context.getEnvironment().getProperty("spring.main.banner-mode"));
            assertNull(rootLogger().getAppender("SVAROG"));
            assertNotNull(rootLogger().getAppender("CONSOLE"));
        }
    }

    @Test
    public void keepsApplicationRunningWhenBridgeCannotStart() {
        try (var context = run("--svarog.logging.enabled=true",
            "--svarog.logging.bridge-path=" + this.directory.resolve("missing"))) {
            assertTrue(context.isActive());
            assertNull(rootLogger().getAppender("SVAROG"));
            assertNotNull(rootLogger().getAppender("CONSOLE"));
        }
    }

    @Test
    @EnabledOnOs(OS.LINUX)
    public void forwardsLogsAndDrainsTheBridgeWhenTheApplicationCloses() throws Exception {
        var bridge = createBridge();
        Appender<ILoggingEvent> appender;
        try (var context = run("--svarog.logging.enabled=true", "--svarog.logging.bridge-path=" + bridge,
            "--spring.main.lazy-initialization=true")) {
            appender = rootLogger().getAppender("SVAROG");
            assertNotNull(appender);
            assertTrue(appender.isStarted());
            assertNotNull(rootLogger().getAppender("CONSOLE"));
            LoggerFactory.getLogger(this.getClass()).info("forwarded by the Spring Boot starter");
        }

        assertNull(rootLogger().getAppender("SVAROG"));
        assertFalse(appender.isStarted());
        var messages = Files.readString(this.directory.resolve("received"));
        assertTrue(messages.contains(":: Spring Boot ::"));
        assertTrue(messages.contains("forwarded by the Spring Boot starter"));
    }

    @Test
    @EnabledOnOs(OS.LINUX)
    public void drainsTheFinalStartupFailureEvenWhenRefreshFails() throws Exception {
        var bridge = createBridge();
        var console = this.directory.resolve("console");
        // A failed refresh does not publish ContextClosedEvent. Exercise Boot's real JVM shutdown hook.
        var child = new ProcessBuilder(Path.of(System.getProperty("java.home"), "bin", "java").toString(),
            "-cp", System.getProperty("java.class.path"), TestApplication.class.getName(),
            "--svarog.logging.enabled=true", "--svarog.logging.bridge-path=" + bridge,
            "--test.fail-on-startup=true", "--spring.main.banner-mode=off")
            .redirectErrorStream(true).redirectOutput(console.toFile()).start();
        assertTrue(child.waitFor(30, TimeUnit.SECONDS), "Timed out waiting for failed application");
        assertEquals(1, child.exitValue(), Files.readString(console));
        var messages = Files.readString(this.directory.resolve("received"));
        assertFalse(messages.contains(":: Spring Boot ::"));
        assertTrue(messages.contains("Starting "), messages);
        assertTrue(messages.contains("Application run failed"), messages);
        assertTrue(messages.contains("Caused by: java.lang.IllegalStateException: startup probe"), messages);
    }

    private Path createBridge() throws IOException {
        var bridge = Files.createFile(this.directory.resolve("bridge"),
            PosixFilePermissions.asFileAttribute(PosixFilePermissions.fromString("rwx------")));
        return Files.writeString(bridge, """
            #!/bin/sh
            printf '%s\\n' '{"v":1,"op":"ready"}'
            cd "$(dirname "$0")" || exit 1
            cat > receiving && mv receiving received
            """);
    }

    private static ConfigurableApplicationContext run(String... args) {
        var environment = new StandardEnvironment();
        environment.getPropertySources().remove(StandardEnvironment.SYSTEM_ENVIRONMENT_PROPERTY_SOURCE_NAME);
        return new SpringApplicationBuilder(TestApplication.class)
            .environment(environment)
            .web(WebApplicationType.NONE)
            .properties("spring.config.name=svarog-starter-test")
            .run(args);
    }

    private static Logger rootLogger() {
        return ((LoggerContext) LoggerFactory.getILoggerFactory()).getLogger(Logger.ROOT_LOGGER_NAME);
    }

    @Configuration(proxyBeanMethods = false)
    @EnableAutoConfiguration
    private static class TestApplication {
        public static void main(String[] args) {
            run(args);
        }

        @Bean
        @ConditionalOnProperty(name = "test.fail-on-startup")
        String failingBean() {
            throw new IllegalStateException("startup probe");
        }
    }
}
