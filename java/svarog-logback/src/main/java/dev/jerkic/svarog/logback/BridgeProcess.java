package dev.jerkic.svarog.logback;

import ch.qos.logback.core.Context;
import ch.qos.logback.core.spi.ContextAwareBase;
import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonToken;
import java.io.IOException;
import java.io.InputStream;
import java.lang.ProcessBuilder.Redirect;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.ArrayDeque;
import java.util.concurrent.TimeUnit;

public final class BridgeProcess extends ContextAwareBase {
    private static final JsonFactory JSON = new JsonFactory();
    private static final int MAX_READY_BYTES = 1024;
    private static final long READY_NANOS = TimeUnit.SECONDS.toNanos(5);
    private static final long DRAIN_NANOS = TimeUnit.SECONDS.toNanos(5);
    private static final long ABORT_NANOS = TimeUnit.SECONDS.toNanos(1);
    private static final long DROP_WARNING_NANOS = TimeUnit.SECONDS.toNanos(10);
    private static final int QUEUE_CAPACITY = 128;
    private static final int QUEUE_BYTES = BridgeFrame.MAX_BYTES;

    private final Process process;
    private final ProcessHandle handle;
    private final Thread worker;
    private final Path extracted;
    private final ArrayDeque<byte[]> queue = new ArrayDeque<>();
    private volatile boolean accepting;
    private volatile boolean aborting;
    private volatile boolean exitExpected;
    private int queuedBytes;
    private long dropped;
    private long nextDropWarning;

    private BridgeProcess(Process process, Path extracted, Context context) {
        this.setContext(context);
        this.process = process;
        this.extracted = extracted;
        this.handle = process.toHandle();
        this.worker = new Thread(this::writeFrames, "svarog-bridge-" + this.handle.pid());
        this.worker.setDaemon(true);
    }

    public static BridgeProcess launch(String bridgePath, Context context) throws IOException {
        var executable = bridgePath;
        if (executable == null || executable.isBlank()) {
            executable = System.getenv("SVAROG_BRIDGE_PATH");
        }
        Path extracted = null;
        try {
            if (executable == null || executable.isBlank()) {
                try (var binary = openBundledBridge()) {
                    extracted = Files.createTempFile("svarog-bridge-", "",
                        PosixFilePermissions.asFileAttribute(PosixFilePermissions.fromString("rwx------")));
                    try (var output = Files.newOutputStream(extracted)) {
                        binary.transferTo(output);
                    }
                }
                executable = extracted.toString();
            }
            var process = new ProcessBuilder(executable).redirectError(Redirect.INHERIT).start();
            return new BridgeProcess(process, extracted, context);
        } catch (IOException e) {
            if (extracted != null) {
                try {
                    Files.deleteIfExists(extracted);
                } catch (IOException cleanupError) {
                    e.addSuppressed(cleanupError);
                }
            }
            throw e;
        }
    }

    public void start() throws IOException, InterruptedException {
        try {
            this.awaitReady();
        } finally {
            // Linux can unlink a running executable. Wait for the child's ready line first.
            if (this.extracted != null) {
                try {
                    Files.deleteIfExists(this.extracted);
                } catch (IOException e) {
                    this.addWarn("Failed to delete extracted Svarog bridge", e);
                }
            }
        }
        this.accepting = true;
        this.handle.onExit().thenRun(this::childExited);
        this.worker.start();
    }

    public boolean isAccepting() {
        return this.accepting;
    }

    public boolean isTerminated() {
        return !this.handle.isAlive() && !this.worker.isAlive();
    }

    public boolean tryOffer(byte[] frame) {
        synchronized (this) {
            if (!this.accepting) {
                return false;
            }
            if (this.queue.size() < QUEUE_CAPACITY && frame.length <= QUEUE_BYTES - this.queuedBytes) {
                this.queue.addLast(frame);
                this.queuedBytes += frame.length;
                this.notifyAll();
                return true;
            }
        }
        this.reportDrop("bridge queue full");
        return false;
    }

    public void reportDrop(String reason) {
        long count;
        synchronized (this) {
            this.dropped++;
            var now = System.nanoTime();
            if (this.nextDropWarning != 0 && now - this.nextDropWarning < 0) {
                return;
            }
            this.nextDropWarning = now + DROP_WARNING_NANOS;
            count = this.dropped;
            this.dropped = 0;
        }
        this.addWarn("Svarog dropped " + count + " log record(s); latest reason: " + reason);
    }

    public void stop() {
        this.closeAdmission(false);
        try {
            if (!this.awaitTermination(System.nanoTime() + DRAIN_NANOS)) {
                this.abort();
                this.addError("Svarog bridge did not drain before the shutdown deadline; delivery is not confirmed");
            } else if (this.process.exitValue() != 0) {
                this.addError("Svarog bridge exited with code " + this.process.exitValue() + "; delivery is not confirmed");
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            this.abort();
            this.addError("Interrupted while stopping Svarog bridge; delivery is not confirmed", e);
        } finally {
            long count;
            synchronized (this) {
                count = this.dropped;
                this.dropped = 0;
            }
            if (count > 0) {
                this.addWarn("Svarog dropped " + count + " additional log record(s)");
            }
        }
    }

    public void abort() {
        this.aborting = true;
        this.terminateChild();
        var deadline = System.nanoTime() + ABORT_NANOS;
        var interrupted = Thread.interrupted();
        try {
            while (true) {
                try {
                    if (!this.awaitTermination(deadline)) {
                        this.addError("Could not confirm termination of Svarog bridge");
                    }
                    return;
                } catch (InterruptedException e) {
                    interrupted = true;
                }
            }
        } finally {
            if (interrupted) {
                Thread.currentThread().interrupt();
            }
        }
    }

    private static InputStream openBundledBridge() throws IOException {
        var os = System.getProperty("os.name");
        var arch = System.getProperty("os.arch");
        if ("Linux".equalsIgnoreCase(os)) {
            var resourceArch = switch (arch) {
                case "amd64", "x86_64" -> "amd64";
                case "aarch64", "arm64" -> "arm64";
                default -> null;
            };
            if (resourceArch != null) {
                var binary = BridgeProcess.class.getResourceAsStream("native/svarog-bridge-linux-" + resourceArch);
                if (binary != null) {
                    return binary;
                }
            }
        }
        throw new IOException("No bundled Svarog bridge for " + os + "/" + arch + "; set SVAROG_BRIDGE_PATH");
    }

    private void awaitReady() throws IOException, InterruptedException {
        var deadline = System.nanoTime() + READY_NANOS;
        var line = new byte[MAX_READY_BYTES];
        var length = 0;
        try (var input = this.process.getInputStream()) {
            while (System.nanoTime() - deadline < 0) {
                if (Thread.interrupted()) {
                    throw new InterruptedException();
                }
                // Only read available bytes: interrupt alone won't unblock a pipe read.
                if (input.available() > 0) {
                    var value = input.read();
                    if (value == -1) {
                        throw new IOException("Svarog bridge closed stdout before ready");
                    }
                    if (value == '\n') {
                        validateReady(line, length);
                        if (!this.handle.isAlive()) {
                            throw new IOException("Svarog bridge exited during startup");
                        }
                        return;
                    }
                    if (length == line.length) {
                        throw new IOException("Svarog bridge ready response exceeds 1024 bytes");
                    }
                    line[length++] = (byte) value;
                } else if (!this.handle.isAlive()) {
                    throw new IOException("Svarog bridge exited before ready");
                } else {
                    Thread.sleep(10);
                }
            }
        }
        throw new IOException("Timed out waiting for Svarog bridge ready after 5 seconds");
    }

    private static void validateReady(byte[] line, int length) throws IOException {
        try (var parser = JSON.createParser(line, 0, length)) {
            if (parser.nextToken() != JsonToken.START_OBJECT) {
                throw new IOException("Invalid Svarog bridge ready response");
            }
            var version = false;
            var ready = false;
            while (parser.nextToken() == JsonToken.FIELD_NAME) {
                var field = parser.currentName();
                var token = parser.nextToken();
                if ("v".equals(field) && !version && token == JsonToken.VALUE_NUMBER_INT
                    && parser.getIntValue() == 1) {
                    version = true;
                } else if ("op".equals(field) && !ready && token == JsonToken.VALUE_STRING
                    && "ready".equals(parser.getText())) {
                    ready = true;
                } else {
                    throw new IOException("Invalid Svarog bridge ready response");
                }
            }
            if (!version || !ready || parser.currentToken() != JsonToken.END_OBJECT
                || parser.nextToken() != null) {
                throw new IOException("Invalid Svarog bridge ready response");
            }
        }
    }

    private synchronized byte[] take() throws InterruptedException {
        while (this.accepting && this.queue.isEmpty()) {
            this.wait();
        }
        var frame = this.queue.pollFirst();
        if (frame != null) {
            this.queuedBytes -= frame.length;
        }
        return frame;
    }

    private void writeFrames() {
        try {
            var output = this.process.getOutputStream();
            byte[] frame;
            while ((frame = this.take()) != null) {
                output.write(frame);
                output.flush();
            }
            this.exitExpected = true;
            output.close(); // EOF tells the bridge to drain the Go publisher.
        } catch (IOException | InterruptedException e) {
            this.terminateChild();
            if (!this.aborting) {
                this.addError("Svarog bridge write failed; queued records were not delivered", e);
            }
            if (e instanceof InterruptedException) {
                Thread.currentThread().interrupt();
            }
        }
    }

    private void childExited() {
        // Closing admission only starts a drain. The worker may still have frames to send.
        var unexpected = !this.exitExpected;
        this.closeAdmission(true);
        if (unexpected) {
            this.addError("Svarog bridge exited unexpectedly; delivery is not confirmed");
        }
    }

    private synchronized void closeAdmission(boolean discard) {
        this.accepting = false;
        if (discard) {
            this.queue.clear();
            this.queuedBytes = 0;
        }
        this.notifyAll();
    }

    private void terminateChild() {
        this.exitExpected = true;
        this.closeAdmission(true);
        // Process.destroyForcibly() also closes streams and can wait for a blocked writer.
        this.handle.destroyForcibly();
    }

    private boolean awaitTermination(long deadline) throws InterruptedException {
        if (!this.process.waitFor(deadline - System.nanoTime(), TimeUnit.NANOSECONDS)) {
            return false;
        }
        TimeUnit.NANOSECONDS.timedJoin(this.worker, deadline - System.nanoTime());
        return !this.worker.isAlive();
    }
}
