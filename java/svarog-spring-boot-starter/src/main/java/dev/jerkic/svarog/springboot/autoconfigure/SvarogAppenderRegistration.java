package dev.jerkic.svarog.springboot.autoconfigure;

import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.LoggerContext;
import dev.jerkic.svarog.logback.SvarogAppender;
import org.slf4j.LoggerFactory;

public final class SvarogAppenderRegistration {
    private static final String APPENDER_NAME = "SVAROG";
    private final String bridgePath;
    private Logger root;
    private SvarogAppender appender;

    public SvarogAppenderRegistration(String bridgePath) {
        this.bridgePath = bridgePath;
    }

    public void start() {
        if (!(LoggerFactory.getILoggerFactory() instanceof LoggerContext context)) {
            return;
        }

        var rootLogger = context.getLogger(Logger.ROOT_LOGGER_NAME);
        synchronized (rootLogger) {
            if (hasSvarogAppender(rootLogger)) {
                return;
            }

            var candidate = new SvarogAppender();
            candidate.setName(APPENDER_NAME);
            candidate.setContext(context);
            candidate.setBridgePath(this.bridgePath);
            candidate.start();
            if (!candidate.isStarted()) {
                candidate.stop();
                LoggerFactory.getLogger(SvarogAppenderRegistration.class).warn(
                    "Svarog log forwarding did not start. Check SVAROG_CONN_STRING and the bridge executable.");
                return;
            }

            rootLogger.addAppender(candidate);
            this.root = rootLogger;
            this.appender = candidate;
        }
    }

    public void close() {
        if (this.appender != null) {
            this.root.detachAppender(this.appender);
            this.appender.stop();
        }
    }

    private static boolean hasSvarogAppender(Logger rootLogger) {
        var appenders = rootLogger.iteratorForAppenders();
        while (appenders.hasNext()) {
            var appender = appenders.next();
            if (appender instanceof SvarogAppender || APPENDER_NAME.equals(appender.getName())) {
                return true;
            }
        }
        return false;
    }
}
