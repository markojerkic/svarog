package dev.jerkic.svarog.logback;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTimeoutPreemptively;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assumptions.assumeTrue;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.LoggerContext;
import ch.qos.logback.classic.spi.LoggingEvent;
import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonToken;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CountDownLatch;
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
public class SvarogAppenderTest {
    @TempDir
    public Path directory;

    private LoggerContext context;
    private SvarogAppender appender;

    @BeforeEach
    public void setUp() {
        this.context = new LoggerContext();
        this.context.start();
        this.appender = new SvarogAppender();
        this.appender.setContext(this.context);
    }

    @AfterEach
    public void cleanUp() throws Exception {
        this.appender.stop();
        this.context.stop();
        // Keep a failed lifecycle assertion from leaving its fixture running.
        var pid = this.directory.resolve("pid");
        if (Files.exists(pid)) {
            var process = ProcessHandle.of(Long.parseLong(Files.readString(pid))).orElse(null);
            if (process != null && process.isAlive()) {
                process.destroyForcibly();
                process.onExit().get(5, TimeUnit.SECONDS);
            }
        }
    }

    @Test
    @EnabledOnOs(value = OS.LINUX, architectures = {"amd64", "x86_64"})
    public void startsBundledBridgeWithoutAnOverride() {
        var override = System.getenv("SVAROG_BRIDGE_PATH");
        assumeTrue(override == null || override.isBlank());
        this.appender.start();
        assertTrue(this.appender.isStarted(), "the bundled bridge must be extracted and executable");
    }

    @Test
    public void acceptsLogsAfterReadyAndStopsTheSameHelperOnEof() throws Exception {
        var received = this.directory.resolve("received");
        this.helper("printf '%s\\n' ' {\"op\":\"ready\", \"v\":1} '\n"
            + "while IFS= read -r line; do printf '%s\\n' \"$line\" >> '" + received + "'; done\n");

        this.appender.start();
        assertTrue(this.appender.isStarted(), "a valid ready response must enable log admission");
        var pid = this.helperPid();
        this.appender.start();
        assertEquals(pid, this.helperPid(), "repeated start must reuse the already-ready helper");
        var event = event("accepted after ready");
        this.appender.doAppend(event);

        var failure = new IllegalStateException("save failed", new IOException("database offline"));
        failure.addSuppressed(new IOException("cleanup failed"));
        var error = new LoggingEvent(this.getClass().getName(), this.context.getLogger("example.Worker"),
            Level.ERROR, "Failed request {}", failure, new Object[] {"abc\"č\nxyz"});
        error.setInstant(event.getInstant());
        this.appender.doAppend(error);
        this.appender.stop();
        this.appender.stop();

        var records = Files.readAllLines(received);
        assertEquals(2, records.size(), "a multiline exception must remain one JSON record alongside the normal event");
        assertEquals("{\"message\":\"INFO example.Worker - accepted after ready\",\"timestamp\":\"2026-11-16T10:00:00.123456789Z\"}",
            records.get(0), "normal logs must retain severity, logger, source message and original timestamp");
        try (var parser = new JsonFactory().createParser(records.get(1))) {
            assertEquals(JsonToken.START_OBJECT, parser.nextToken(), "the error must remain valid JSON");
            assertEquals("message", parser.nextFieldName(), "the existing message field must carry all logging details");
            assertEquals(JsonToken.VALUE_STRING, parser.nextToken(), "error details must be a single string");
            var message = parser.getText();
            assertTrue(message.startsWith("ERROR example.Worker - Failed request abc\"č\nxyz\n"),
                "error records must preserve severity, logger, substituted arguments and Unicode");
            assertTrue(message.contains("java.lang.IllegalStateException: save failed"),
                "the original exception type and message must survive logging");
            assertTrue(message.contains("at " + this.getClass().getName() + "."),
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
        this.assertHelperExited();
        this.appender.start();
        assertTrue(this.appender.isStarted(), "the same appender must start a new bridge after stop");
        assertTrue(pid != this.helperPid(), "restarting must not reuse the stopped process");
        this.appender.stop();
        this.assertHelperExited();
    }

    @Test
    public void remainsResponsiveAndStopsWhenTheBridgeNeverReads() throws Exception {
        this.helper("printf '%s\\n' '{\"v\":1,\"op\":\"ready\"}'\nexec sleep 30\n");
        this.appender.start();
        assertTrue(this.appender.isStarted());

        var event = event("x".repeat(1024 * 1024));
        assertTimeoutPreemptively(Duration.ofSeconds(4), () -> {
            for (var i = 0; i < 12; i++) {
                this.appender.doAppend(event);
            }
        }, "a full pipe and queue must not block callers");
        assertTrue(this.context.getStatusManager().getCopyOfStatusList().stream()
            .anyMatch(status -> status.getMessage().contains("bridge queue full")));

        assertTimeoutPreemptively(Duration.ofSeconds(8), this.appender::stop);
        this.assertHelperExited();
    }

    @Test
    public void admitsOnlyAfterReadyAndReportsChildExitDuringDrain() throws Exception {
        var exit = this.directory.resolve("exit");
        var read = this.directory.resolve("read");
        var reported = new CountDownLatch(1);
        this.context.getStatusManager().add(status -> {
            if (status.getMessage().contains("bridge exited unexpectedly")) {
                reported.countDown();
            }
        });
        // Keep the pipe open after the helper exits so the writer cannot diagnose the loss via EPIPE.
        var path = this.helper("exec 3<&0\n"
            + "(while [ ! -f '" + read + "' ]; do sleep .01; done; exec cat >/dev/null) <&3 &\n"
            + "printf '%s\\n' '{\"v\":1,\"op\":\"ready\"}'\n"
            + "while [ ! -f '" + exit + "' ]; do sleep .01; done\n");
        var bridge = BridgeProcess.launch(path, this.context);
        CompletableFuture<Void> stopped;
        try {
            var record = BridgeFrame.encode("queued", Instant.now());
            assertFalse(bridge.tryOffer(record), "a bridge still starting must reject late calls from the previous run");
            bridge.start();
            assertTrue(bridge.tryOffer(BridgeFrame.encode("x".repeat(2 * 1024 * 1024), Instant.now())));
            assertTrue(bridge.tryOffer(record));

            stopped = CompletableFuture.runAsync(bridge::stop);
            assertTimeoutPreemptively(Duration.ofSeconds(3), () -> {
                while (bridge.isAccepting()) {
                    Thread.sleep(10);
                }
            });
            Files.createFile(exit);
            assertTrue(reported.await(3, TimeUnit.SECONDS),
                "a zero exit before the worker drains the queue must be reported");
        } finally {
            Files.writeString(exit, "");
            Files.writeString(read, "");
        }
        stopped.get(8, TimeUnit.SECONDS);
        this.assertHelperExited();
    }

    @Test
    public void interruptedStopStillTerminatesTheBridge() throws Exception {
        this.helper("printf '%s\\n' '{\"v\":1,\"op\":\"ready\"}'\nexec sleep 30\n");
        this.appender.start();
        assertTrue(this.appender.isStarted());

        assertTimeoutPreemptively(Duration.ofSeconds(3), () -> {
            Thread.currentThread().interrupt();
            try {
                this.appender.stop();
                assertTrue(Thread.currentThread().isInterrupted());
            } finally {
                Thread.interrupted();
            }
        });
        this.assertHelperExited();
    }

    @Test
    public void rejectsInvalidAndOversizedRecordsWithoutStoppingTheBridge() throws Exception {
        var received = this.directory.resolve("received");
        this.helper("printf '%s\\n' '{\"v\":1,\"op\":\"ready\"}'\n"
            + "while IFS= read -r line; do printf '%s\\n' \"$line\" >> '" + received + "'; done\n");
        this.appender.start();
        assertTrue(this.appender.isStarted());

        this.appender.doAppend(event("contains \0 NUL"));
        this.appender.doAppend(event("\u0001".repeat(BridgeFrame.MAX_BYTES / 6)));
        var invalidTime = event("unrepresentable timestamp");
        invalidTime.setInstant(Instant.MIN);
        this.appender.doAppend(invalidTime);
        this.appender.doAppend(event("still delivered"));
        this.appender.stop();

        assertEquals(List.of(
            "{\"message\":\"INFO example.Worker - still delivered\",\"timestamp\":\"2026-11-16T10:00:00.123456789Z\"}"),
            Files.readAllLines(received));
        this.assertHelperExited();
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
    public void rejectsInvalidReadyAndReapsTheOwnedHelper(String response) throws Exception {
        this.helper("printf '%s\\n' '" + response + "'\nIFS= read -r line\n");
        this.appender.start();

        assertFalse(this.appender.isStarted(), "an incompatible or malformed response must not enable logging");
        this.assertHelperExited();
    }

    @Test
    public void rejectsAnOversizedUnterminatedResponse() throws Exception {
        this.helper("printf '%s' '" + "x".repeat(1025) + "'\nIFS= read -r line\n");
        this.appender.start();

        assertFalse(this.appender.isStarted(), "an oversized response must fail without waiting for a newline");
        this.assertHelperExited();
    }

    @Test
    public void rejectsEofBeforeACompleteReadyLine() throws Exception {
        this.helper("printf '%s' '{\"v\":1,\"op\":\"ready\"}'\n");
        this.appender.start();

        assertFalse(this.appender.isStarted(), "EOF must not substitute for the ready line's terminating newline");
        this.assertHelperExited();
    }

    @Test
    public void timesOutAndReapsAnUnresponsiveHelper() throws Exception {
        this.helper("IFS= read -r line\n");
        var started = System.nanoTime();
        this.appender.start();
        var elapsed = Duration.ofNanos(System.nanoTime() - started);

        assertFalse(this.appender.isStarted(), "a silent helper must never enable log admission");
        assertTrue(elapsed.compareTo(Duration.ofMillis(4500)) >= 0, "startup must allow the ready deadline");
        assertTrue(elapsed.compareTo(Duration.ofSeconds(8)) < 0, "startup must not block indefinitely on stdout");
        this.assertHelperExited();
    }

    private String helper(String body) throws IOException {
        var script = this.directory.resolve("helper");
        Files.writeString(script, "#!/bin/sh\nprintf '%s' \"$$\" > '" + this.directory.resolve("pid") + "'\n" + body);
        Files.setPosixFilePermissions(script, PosixFilePermissions.fromString("rwx------"));
        this.appender.setBridgePath(script.toString());
        return script.toString();
    }

    private static LoggingEvent event(String message) {
        var event = new LoggingEvent();
        event.setLevel(Level.INFO);
        event.setLoggerName("example.Worker");
        event.setMessage(message);
        event.setInstant(Instant.parse("2026-11-16T10:00:00.123456789Z"));
        return event;
    }

    private long helperPid() throws IOException {
        return Long.parseLong(Files.readString(this.directory.resolve("pid")));
    }

    private void assertHelperExited() throws IOException {
        assertFalse(ProcessHandle.of(this.helperPid()).map(ProcessHandle::isAlive).orElse(false),
            "startup failure or EOF shutdown must reap the appender's own child");
    }
}
