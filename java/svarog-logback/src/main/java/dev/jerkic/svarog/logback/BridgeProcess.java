package dev.jerkic.svarog.logback;

import ch.qos.logback.core.Context;
import ch.qos.logback.core.spi.ContextAwareBase;
import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonToken;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.lang.ProcessBuilder.Redirect;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.ArrayDeque;
import java.util.concurrent.TimeUnit;

final class BridgeProcess extends ContextAwareBase {
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
        setContext(context);
        this.process = process;
        this.extracted = extracted;
        handle = process.toHandle();
        worker = new Thread(this::writeFrames, "svarog-bridge-" + handle.pid());
        worker.setDaemon(true);
    }

    static BridgeProcess launch(String bridgePath, Context context) throws IOException {
        String executable = bridgePath;
        if (executable == null || executable.isBlank()) {
            executable = System.getenv("SVAROG_BRIDGE_PATH");
        }
        Path extracted = null;
        try {
            if (executable == null || executable.isBlank()) {
                try (InputStream binary = openBundledBridge()) {
                    extracted = Files.createTempFile("svarog-bridge-", "",
                        PosixFilePermissions.asFileAttribute(PosixFilePermissions.fromString("rwx------")));
                    try (OutputStream output = Files.newOutputStream(extracted)) {
                        binary.transferTo(output);
                    }
                }
                executable = extracted.toString();
            }
            Process process = new ProcessBuilder(executable).redirectError(Redirect.INHERIT).start();
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

    private static InputStream openBundledBridge() throws IOException {
        String os = System.getProperty("os.name");
        String arch = System.getProperty("os.arch");
        if ("Linux".equalsIgnoreCase(os)) {
            String resourceArch = switch (arch) {
                case "amd64", "x86_64" -> "amd64";
                case "aarch64", "arm64" -> "arm64";
                default -> null;
            };
            if (resourceArch != null) {
                InputStream binary = BridgeProcess.class.getResourceAsStream("native/svarog-bridge-linux-" + resourceArch);
                if (binary != null) {
                    return binary;
                }
            }
        }
        throw new IOException("No bundled Svarog bridge for " + os + "/" + arch + "; set SVAROG_BRIDGE_PATH");
    }

    void start() throws IOException, InterruptedException {
        try {
            awaitReady();
        } finally {
            // Linux can unlink a running executable. Wait for the child's ready line first.
            if (extracted != null) {
                try {
                    Files.deleteIfExists(extracted);
                } catch (IOException e) {
                    addWarn("Failed to delete extracted Svarog bridge", e);
                }
            }
        }
        accepting = true;
        handle.onExit().thenRun(this::childExited);
        worker.start();
    }

    private void awaitReady() throws IOException, InterruptedException {
        long deadline = System.nanoTime() + READY_NANOS;
        byte[] line = new byte[MAX_READY_BYTES];
        int length = 0;
        try (InputStream input = process.getInputStream()) {
            while (System.nanoTime() - deadline < 0) {
                if (Thread.interrupted()) {
                    throw new InterruptedException();
                }
                // Only read available bytes: interrupt alone won't unblock a pipe read.
                if (input.available() > 0) {
                    int value = input.read();
                    if (value == -1) {
                        throw new IOException("Svarog bridge closed stdout before ready");
                    }
                    if (value == '\n') {
                        validateReady(line, length);
                        if (!handle.isAlive()) {
                            throw new IOException("Svarog bridge exited during startup");
                        }
                        return;
                    }
                    if (length == line.length) {
                        throw new IOException("Svarog bridge ready response exceeds 1024 bytes");
                    }
                    line[length++] = (byte) value;
                } else if (!handle.isAlive()) {
                    throw new IOException("Svarog bridge exited before ready");
                } else {
                    Thread.sleep(10);
                }
            }
        }
        throw new IOException("Timed out waiting for Svarog bridge ready after 5 seconds");
    }

    private static void validateReady(byte[] line, int length) throws IOException {
        try (JsonParser parser = BridgeFrame.JSON.createParser(line, 0, length)) {
            if (parser.nextToken() != JsonToken.START_OBJECT) {
                throw new IOException("Invalid Svarog bridge ready response");
            }
            boolean version = false;
            boolean ready = false;
            while (parser.nextToken() == JsonToken.FIELD_NAME) {
                String field = parser.currentName();
                JsonToken token = parser.nextToken();
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

    boolean isAccepting() {
        return accepting;
    }

    boolean isTerminated() {
        return !handle.isAlive() && !worker.isAlive();
    }

    boolean tryOffer(byte[] frame) {
        synchronized (this) {
            if (!accepting) {
                return false;
            }
            if (queue.size() < QUEUE_CAPACITY && frame.length <= QUEUE_BYTES - queuedBytes) {
                queue.addLast(frame);
                queuedBytes += frame.length;
                notifyAll();
                return true;
            }
        }
        reportDrop("bridge queue full");
        return false;
    }

    void reportDrop(String reason) {
        long count;
        synchronized (this) {
            dropped++;
            long now = System.nanoTime();
            if (nextDropWarning != 0 && now - nextDropWarning < 0) {
                return;
            }
            nextDropWarning = now + DROP_WARNING_NANOS;
            count = dropped;
            dropped = 0;
        }
        addWarn("Svarog dropped " + count + " log record(s); latest reason: " + reason);
    }

    private synchronized byte[] take() throws InterruptedException {
        while (accepting && queue.isEmpty()) {
            wait();
        }
        byte[] frame = queue.pollFirst();
        if (frame != null) {
            queuedBytes -= frame.length;
        }
        return frame;
    }

    private void writeFrames() {
        try {
            OutputStream output = process.getOutputStream();
            byte[] frame;
            while ((frame = take()) != null) {
                output.write(frame);
                output.flush();
            }
            exitExpected = true;
            output.close(); // EOF tells the bridge to drain the Go publisher.
        } catch (IOException | InterruptedException e) {
            terminateChild();
            if (!aborting) {
                addError("Svarog bridge write failed; queued records were not delivered", e);
            }
            if (e instanceof InterruptedException) {
                Thread.currentThread().interrupt();
            }
        }
    }

    private void childExited() {
        // Closing admission only starts a drain. The worker may still have frames to send.
        boolean unexpected = !exitExpected;
        closeAdmission(true);
        if (unexpected) {
            addError("Svarog bridge exited unexpectedly; delivery is not confirmed");
        }
    }

    private synchronized void closeAdmission(boolean discard) {
        accepting = false;
        if (discard) {
            queue.clear();
            queuedBytes = 0;
        }
        notifyAll();
    }

    void stop() {
        closeAdmission(false);
        try {
            if (!awaitTermination(System.nanoTime() + DRAIN_NANOS)) {
                abort();
                addError("Svarog bridge did not drain before the shutdown deadline; delivery is not confirmed");
            } else if (process.exitValue() != 0) {
                addError("Svarog bridge exited with code " + process.exitValue() + "; delivery is not confirmed");
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            abort();
            addError("Interrupted while stopping Svarog bridge; delivery is not confirmed", e);
        } finally {
            long count;
            synchronized (this) {
                count = dropped;
                dropped = 0;
            }
            if (count > 0) {
                addWarn("Svarog dropped " + count + " additional log record(s)");
            }
        }
    }

    void abort() {
        aborting = true;
        terminateChild();
        long deadline = System.nanoTime() + ABORT_NANOS;
        boolean interrupted = Thread.interrupted();
        try {
            while (true) {
                try {
                    if (!awaitTermination(deadline)) {
                        addError("Could not confirm termination of Svarog bridge");
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

    private void terminateChild() {
        exitExpected = true;
        closeAdmission(true);
        // Process.destroyForcibly() also closes streams and can wait for a blocked writer.
        handle.destroyForcibly();
    }

    private boolean awaitTermination(long deadline) throws InterruptedException {
        if (!process.waitFor(deadline - System.nanoTime(), TimeUnit.NANOSECONDS)) {
            return false;
        }
        TimeUnit.NANOSECONDS.timedJoin(worker, deadline - System.nanoTime());
        return !worker.isAlive();
    }
}
