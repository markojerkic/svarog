package dev.jerkic.svarog.logback;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.LoggerContext;
import ch.qos.logback.classic.spi.LoggingEvent;
import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonToken;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
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
        event.setLevel(Level.INFO);
        event.setLoggerName("example.Worker");
        event.setMessage("accepted after ready");
        event.setInstant(Instant.parse("2026-11-16T10:00:00.123456789Z"));
        appender.doAppend(event);

        IllegalStateException failure = new IllegalStateException("save failed", new IOException("database offline"));
        failure.addSuppressed(new IOException("cleanup failed"));
        LoggingEvent error = new LoggingEvent(getClass().getName(), context.getLogger("example.Worker"),
            Level.ERROR, "Failed request {}", failure, new Object[] {"abc\"č\nxyz"});
        error.setInstant(event.getInstant());
        appender.doAppend(error);
        appender.stop();
        appender.stop();

        List<String> records = Files.readAllLines(received);
        assertEquals(2, records.size(), "a multiline exception must remain one JSON record alongside the normal event");
        assertEquals("{\"message\":\"INFO example.Worker - accepted after ready\",\"timestamp\":\"2026-11-16T10:00:00.123456789Z\"}",
            records.get(0), "normal logs must retain severity, logger, source message and original timestamp");
        try (JsonParser parser = new JsonFactory().createParser(records.get(1))) {
            assertEquals(JsonToken.START_OBJECT, parser.nextToken(), "the error must remain valid JSON");
            assertEquals("message", parser.nextFieldName(), "the existing message field must carry all logging details");
            assertEquals(JsonToken.VALUE_STRING, parser.nextToken(), "error details must be a single string");
            String message = parser.getText();
            assertTrue(message.startsWith("ERROR example.Worker - Failed request abc\"č\nxyz\n"),
                "error records must preserve severity, logger, substituted arguments and Unicode");
            assertTrue(message.contains("java.lang.IllegalStateException: save failed"),
                "the original exception type and message must survive logging");
            assertTrue(message.contains("at " + getClass().getName() + "."),
                "exception stack frames must reach the helper rather than only an exception summary");
            assertTrue(message.contains("Caused by: java.io.IOException: database offline"),
                "nested exception causes must not be lost");
            assertTrue(message.contains("Suppressed: java.io.IOException: cleanup failed"),
                "suppressed exceptions must not be lost");
            assertEquals("timestamp", parser.nextFieldName(), "error timestamps must keep the existing protocol field");
            assertEquals(event.getInstant().toString(), parser.nextTextValue(), "error records must retain source nanoseconds");
            assertEquals(JsonToken.END_OBJECT, parser.nextToken(), "metadata must not introduce extra protocol fields");
            assertEquals(null, parser.nextToken(), "an error must not introduce another JSON record");
        }
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
