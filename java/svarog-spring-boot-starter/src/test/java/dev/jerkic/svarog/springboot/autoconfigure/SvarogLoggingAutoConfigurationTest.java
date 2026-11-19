package dev.jerkic.svarog.springboot.autoconfigure;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;

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
import org.springframework.boot.builder.SpringApplicationBuilder;
import org.springframework.context.ConfigurableApplicationContext;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.StandardEnvironment;

public class SvarogLoggingAutoConfigurationTest {
    @TempDir
    public Path directory;

    @Test
    public void leavesDefaultLoggingAloneWhenNotEnabled() {
        try (var context = run("--svarog.logging.bridge-path=" + this.directory.resolve("missing"))) {
            assertFalse(context.containsBean("svarogAppenderRegistration"));
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
        var bridge = Files.createFile(this.directory.resolve("bridge"),
            PosixFilePermissions.asFileAttribute(PosixFilePermissions.fromString("rwx------")));
        Files.writeString(bridge, """
            #!/bin/sh
            printf '%s\\n' '{"v":1,"op":"ready"}'
            exec cat > "$(dirname "$0")/received"
            """);

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
        assertTrue(Files.readString(this.directory.resolve("received")).contains("forwarded by the Spring Boot starter"));
    }

    private static ConfigurableApplicationContext run(String... args) {
        var environment = new StandardEnvironment();
        environment.getPropertySources().remove(StandardEnvironment.SYSTEM_ENVIRONMENT_PROPERTY_SOURCE_NAME);
        return new SpringApplicationBuilder(TestApplication.class)
            .environment(environment)
            .web(WebApplicationType.NONE)
            .logStartupInfo(false)
            .properties("spring.main.banner-mode=off", "spring.config.name=svarog-starter-test")
            .run(args);
    }

    private static Logger rootLogger() {
        return ((LoggerContext) LoggerFactory.getILoggerFactory()).getLogger(Logger.ROOT_LOGGER_NAME);
    }

    @Configuration(proxyBeanMethods = false)
    @EnableAutoConfiguration
    private static class TestApplication {
    }
}
