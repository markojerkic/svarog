package dev.jerkic.svarog.logback;

import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.UnsynchronizedAppenderBase;
import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonGenerator;
import java.io.IOException;
import java.lang.ProcessBuilder.Redirect;
import java.util.concurrent.TimeUnit;

public final class DemoAppender extends UnsynchronizedAppenderBase<ILoggingEvent> {
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
                .redirectOutput(Redirect.DISCARD)
                .start();
            writer = new JsonFactory().createGenerator(helper.getOutputStream());
            writer.setRootValueSeparator(null);
            super.start();
        } catch (IOException e) {
            addError("Failed to start Go helper", e);
            closeHelper();
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
