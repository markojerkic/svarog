package dev.jerkic.svarog.logback;

import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.classic.spi.ThrowableProxyUtil;
import ch.qos.logback.core.UnsynchronizedAppenderBase;
import java.io.IOException;

public final class SvarogAppender extends UnsynchronizedAppenderBase<ILoggingEvent> {
    private String bridgePath;
    private volatile BridgeProcess bridge;

    public void setBridgePath(String bridgePath) {
        this.bridgePath = bridgePath;
    }

    @Override
    public boolean isStarted() {
        var current = this.bridge;
        return super.isStarted() && current != null && current.isAccepting();
    }

    @Override
    public synchronized void start() {
        if (this.bridge != null) {
            if (this.bridge.isAccepting()) {
                return;
            }
            if (!this.bridge.isTerminated()) {
                this.addError("Cannot start Svarog bridge while the previous process is shutting down");
                return;
            }
            super.stop();
            this.bridge = null;
        }
        try {
            this.bridge = BridgeProcess.launch(this.bridgePath, this.getContext());
            this.bridge.start();
            super.start();
        } catch (IOException e) {
            this.abortStartup();
            this.addError("Failed to start Svarog bridge", e);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            this.abortStartup();
            this.addError("Interrupted while waiting for Svarog bridge readiness", e);
        }
    }

    @Override
    public synchronized void stop() {
        super.stop();
        if (this.bridge != null) {
            this.bridge.stop();
            if (this.bridge.isTerminated()) {
                this.bridge = null;
            }
        }
    }

    @Override
    protected void append(ILoggingEvent event) {
        var current = this.bridge;
        if (current == null || !current.isAccepting()) {
            return;
        }
        var message = event.getLevel() + " " + event.getLoggerName() + " - " + event.getFormattedMessage();
        var throwable = event.getThrowableProxy();
        if (throwable != null) {
            message += "\n" + ThrowableProxyUtil.asString(throwable);
        }
        try {
            current.tryOffer(BridgeFrame.encode(message, event.getInstant()));
        } catch (IOException e) {
            current.reportDrop("invalid or oversized record");
        }
    }

    private void abortStartup() {
        if (this.bridge != null) {
            this.bridge.abort();
            if (this.bridge.isTerminated()) {
                this.bridge = null;
            }
        }
    }
}
