package dev.jerkic.svarog.logback;

import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.classic.spi.IThrowableProxy;
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
        BridgeProcess current = bridge;
        return super.isStarted() && current != null && current.isAccepting();
    }

    @Override
    public synchronized void start() {
        if (bridge != null) {
            if (bridge.isAccepting()) {
                return;
            }
            if (!bridge.isTerminated()) {
                addError("Cannot start Svarog bridge while the previous process is shutting down");
                return;
            }
            super.stop();
            bridge = null;
        }
        try {
            bridge = BridgeProcess.launch(bridgePath, getContext());
            bridge.start();
            super.start();
        } catch (IOException e) {
            abortStartup();
            addError("Failed to start Svarog bridge", e);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            abortStartup();
            addError("Interrupted while waiting for Svarog bridge readiness", e);
        }
    }

    @Override
    protected void append(ILoggingEvent event) {
        BridgeProcess current = bridge;
        if (current == null || !current.isAccepting()) {
            return;
        }
        String message = event.getLevel() + " " + event.getLoggerName() + " - " + event.getFormattedMessage();
        IThrowableProxy throwable = event.getThrowableProxy();
        if (throwable != null) {
            message += "\n" + ThrowableProxyUtil.asString(throwable);
        }
        try {
            current.tryOffer(BridgeFrame.encode(message, event.getInstant()));
        } catch (IOException e) {
            current.reportDrop("invalid or oversized record");
        }
    }

    @Override
    public synchronized void stop() {
        super.stop();
        if (bridge != null) {
            bridge.stop();
            if (bridge.isTerminated()) {
                bridge = null;
            }
        }
    }

    private void abortStartup() {
        if (bridge != null) {
            bridge.abort();
            if (bridge.isTerminated()) {
                bridge = null;
            }
        }
    }
}
