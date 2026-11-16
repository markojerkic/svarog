package dev.jerkic.svarog.logback;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import ch.qos.logback.classic.LoggerContext;
import ch.qos.logback.classic.spi.LoggingEvent;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.time.Duration;
import java.time.Instant;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledOnOs;
import org.junit.jupiter.api.condition.OS;
import org.junit.jupiter.api.io.TempDir;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

@EnabledOnOs(OS.LINUX)
class DemoAppenderTest {
    @TempDir
    Path directory;

    private LoggerContext context;
    private DemoAppender appender;

    @BeforeEach
    void setUp() {
        context = new LoggerContext();
        context.start();
        appender = new DemoAppender();
        appender.setContext(context);
    }

    @AfterEach
    void cleanUp() throws Exception {
        appender.stop();
        context.stop();
        // Keep a failed lifecycle assertion from leaving its fixture running.
        Path pid = directory.resolve("pid");
        if (Files.exists(pid)) {
            ProcessHandle process = ProcessHandle.of(Long.parseLong(Files.readString(pid))).orElse(null);
            if (process != null && process.isAlive()) {
                process.destroyForcibly();
                process.onExit().get(5, TimeUnit.SECONDS);
            }
        }
    }

    @Test
    void acceptsLogsAfterReadyAndStopsTheSameHelperOnEof() throws Exception {
        Path received = directory.resolve("received");
        helper("printf '%s\\n' ' {\"op\":\"ready\", \"v\":1} '\n"
            + "while IFS= read -r line; do printf '%s\\n' \"$line\" >> '" + received + "'; done\n");

        appender.start();
        assertTrue(appender.isStarted(), "a valid ready response must enable log admission");
        long pid = helperPid();
        appender.start();
        assertEquals(pid, helperPid(), "repeated start must reuse the already-ready helper");
        LoggingEvent event = new LoggingEvent();
        event.setMessage("accepted after ready");
        event.setInstant(Instant.parse("2026-11-16T10:00:00.123456789Z"));
        appender.doAppend(event);
        appender.stop();
        appender.stop();

        assertEquals("{\"message\":\"accepted after ready\",\"timestamp\":\"2026-11-16T10:00:00.123456789Z\"}\n",
            Files.readString(received), "an admitted event must reach the helper before EOF shutdown");
        assertHelperExited();
    }

    @ParameterizedTest
    @ValueSource(strings = {
        "not-json",
        "{\"v\":2,\"op\":\"ready\"}",
        "{\"v\":1,\"op\":\"other\"}",
        "{\"v\":\"1\",\"op\":\"ready\"}",
        "{\"v\":1,\"op\":\"ready\",\"v\":1}",
        "{\"v\":1,\"op\":\"ready\"} {}"
    })
    void rejectsInvalidReadyAndReapsTheOwnedHelper(String response) throws Exception {
        helper("printf '%s\\n' '" + response + "'\nIFS= read -r line\n");
        appender.start();

        assertFalse(appender.isStarted(), "an incompatible or malformed response must not enable logging");
        assertHelperExited();
    }

    @Test
    void rejectsAnOversizedUnterminatedResponse() throws Exception {
        helper("printf '%s' '" + "x".repeat(1025) + "'\nIFS= read -r line\n");
        appender.start();

        assertFalse(appender.isStarted(), "an oversized response must fail without waiting for a newline");
        assertHelperExited();
    }

    @Test
    void rejectsEofBeforeACompleteReadyLine() throws Exception {
        helper("printf '%s' '{\"v\":1,\"op\":\"ready\"}'\n");
        appender.start();

        assertFalse(appender.isStarted(), "EOF must not substitute for the ready line's terminating newline");
        assertHelperExited();
    }

    @Test
    void timesOutAndReapsAnUnresponsiveHelper() throws Exception {
        helper("IFS= read -r line\n");
        long started = System.nanoTime();
        appender.start();
        Duration elapsed = Duration.ofNanos(System.nanoTime() - started);

        assertFalse(appender.isStarted(), "a silent helper must never enable log admission");
        assertTrue(elapsed.compareTo(Duration.ofMillis(4500)) >= 0, "startup must allow the ready deadline");
        assertTrue(elapsed.compareTo(Duration.ofSeconds(8)) < 0, "startup must not block indefinitely on stdout");
        assertHelperExited();
    }

    private void helper(String body) throws IOException {
        Path script = directory.resolve("helper");
        Files.writeString(script, "#!/bin/sh\nprintf '%s' \"$$\" > '" + directory.resolve("pid") + "'\n" + body);
        Files.setPosixFilePermissions(script, PosixFilePermissions.fromString("rwx------"));
        appender.setHelperPath(script.toString());
    }

    private long helperPid() throws IOException {
        return Long.parseLong(Files.readString(directory.resolve("pid")));
    }

    private void assertHelperExited() throws IOException {
        assertFalse(ProcessHandle.of(helperPid()).map(ProcessHandle::isAlive).orElse(false),
            "startup failure or EOF shutdown must reap the appender's own child");
    }
}
