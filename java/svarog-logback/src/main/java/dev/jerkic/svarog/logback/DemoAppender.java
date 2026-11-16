package dev.jerkic.svarog.logback;

import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.UnsynchronizedAppenderBase;
import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonToken;
import java.io.IOException;
import java.io.InputStream;
import java.lang.ProcessBuilder.Redirect;
import java.util.concurrent.TimeUnit;

public final class DemoAppender extends UnsynchronizedAppenderBase<ILoggingEvent> {
    private static final int MAX_READY_BYTES = 1024;
    private static final long READY_TIMEOUT_NANOS = TimeUnit.SECONDS.toNanos(5);

    private String helperPath;
    private Process helper;
    private volatile JsonGenerator writer;

    public void setHelperPath(String helperPath) {
        this.helperPath = helperPath;
    }

    @Override
    public synchronized void start() {
        if (helper != null || isStarted()) {
            return;
        }
        if (helperPath == null || helperPath.isBlank()) {
            addError("Go helper path is required");
            return;
        }
        try {
            helper = new ProcessBuilder(helperPath)
                .redirectError(Redirect.INHERIT)
                .redirectOutput(Redirect.PIPE)
                .start();
            awaitReady();
            writer = new JsonFactory().createGenerator(helper.getOutputStream());
            writer.setRootValueSeparator(null);
            super.start();
        } catch (IOException e) {
            addError("Failed to start Go helper", e);
            abortStartup();
        } catch (InterruptedException e) {
            addError("Interrupted while waiting for Go helper ready", e);
            abortStartup();
            Thread.currentThread().interrupt();
        }
    }

    private void awaitReady() throws IOException, InterruptedException {
        long deadline = System.nanoTime() + READY_TIMEOUT_NANOS;
        byte[] line = new byte[MAX_READY_BYTES];
        int length = 0;
        try (InputStream input = helper.getInputStream()) {
            while (System.nanoTime() - deadline < 0) {
                if (Thread.interrupted()) {
                    throw new InterruptedException();
                }
                // Only read available bytes: a pipe read cannot be cancelled by
                // interrupt alone, and startup must not leave a reader behind.
                if (input.available() > 0) {
                    int value = input.read();
                    if (value == -1) {
                        throw new IOException("Go helper closed stdout before ready");
                    }
                    if (value == '\n') {
                        validateReady(line, length);
                        if (!helper.isAlive()) {
                            throw new IOException("Go helper exited during startup");
                        }
                        return;
                    }
                    if (length == line.length) {
                        throw new IOException("Go helper ready response exceeds 1024 bytes");
                    }
                    line[length++] = (byte) value;
                } else if (!helper.isAlive()) {
                    throw new IOException("Go helper exited before ready");
                } else {
                    Thread.sleep(10);
                }
            }
        }
        throw new IOException("Timed out waiting for Go helper ready after 5 seconds");
    }

    private void validateReady(byte[] line, int length) throws IOException {
        try (JsonParser parser = new JsonFactory().createParser(line, 0, length)) {
            if (parser.nextToken() != JsonToken.START_OBJECT) {
                throw new IOException("Invalid Go helper ready response");
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
                    throw new IOException("Invalid Go helper ready response");
                }
            }
            if (!version || !ready || parser.currentToken() != JsonToken.END_OBJECT
                || parser.nextToken() != null) {
                throw new IOException("Invalid Go helper ready response");
            }
        }
    }

    private void abortStartup() {
        if (helper == null) {
            return;
        }
        // Only the process started by this appender may be terminated.
        helper.destroyForcibly();
        boolean interrupted = Thread.interrupted();
        while (true) {
            try {
                helper.waitFor();
                break;
            } catch (InterruptedException e) {
                interrupted = true;
            }
        }
        closeHelper();
        helper = null;
        if (interrupted) {
            Thread.currentThread().interrupt();
        }
    }

    @Override
    protected void append(ILoggingEvent event) {
        String message = event.getFormattedMessage();
        String timestamp = event.getInstant().toString();
        JsonGenerator output = writer;
        if (output == null) {
            return;
        }
        try {
            synchronized (output) {
                if (!isStarted()) {
                    return;
                }
                output.writeStartObject();
                output.writeStringField("message", message);
                output.writeStringField("timestamp", timestamp);
                output.writeEndObject();
                output.writeRaw('\n');
                output.flush();
            }
        } catch (IOException e) {
            addError("Failed to write log to Go helper", e);
            stop();
        }
    }

    @Override
    public synchronized void stop() {
        if (!isStarted()) {
            return;
        }
        super.stop();
        closeHelper();
    }

    private void closeHelper() {
        if (helper == null) {
            return;
        }
        JsonGenerator output = writer;
        writer = null;
        try {
            if (output != null) {
                synchronized (output) {
                    output.close();
                }
            } else {
                helper.getOutputStream().close();
            }
        } catch (IOException e) {
            addError("Failed to close Go helper input", e);
        }
        try {
            helper.getInputStream().close();
        } catch (IOException e) {
            addError("Failed to close Go helper output", e);
        }
        try {
            if (!helper.waitFor(5, TimeUnit.SECONDS)) {
                addError("Go helper did not exit after EOF");
            } else if (helper.exitValue() != 0) {
                addError("Go helper exited with code " + helper.exitValue());
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            addError("Interrupted while waiting for Go helper", e);
        }
    }
}
