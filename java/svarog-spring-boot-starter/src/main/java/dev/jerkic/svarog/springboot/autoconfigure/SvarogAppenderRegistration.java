package dev.jerkic.svarog.springboot.autoconfigure;

import java.util.Iterator;

import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.LoggerContext;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.Appender;
import org.slf4j.LoggerFactory;

import dev.jerkic.svarog.logback.SvarogAppender;

final class SvarogAppenderRegistration {
    private static final String APPENDER_NAME = "SVAROG";
    private final String bridgePath;
    private Logger root;
    private SvarogAppender appender;

    SvarogAppenderRegistration(String bridgePath) {
        this.bridgePath = bridgePath;
    }

    void start() {
        if (!(LoggerFactory.getILoggerFactory() instanceof LoggerContext context)) {
            return;
        }

        Logger rootLogger = context.getLogger(Logger.ROOT_LOGGER_NAME);
        synchronized (rootLogger) {
            if (hasSvarogAppender(rootLogger)) {
                return;
            }

            SvarogAppender candidate = new SvarogAppender();
            candidate.setName(APPENDER_NAME);
            candidate.setContext(context);
            candidate.setBridgePath(bridgePath);
            candidate.start();
            if (!candidate.isStarted()) {
                candidate.stop();
                LoggerFactory.getLogger(SvarogAppenderRegistration.class).warn(
                    "Svarog log forwarding did not start. Check SVAROG_CONN_STRING and the bridge executable.");
                return;
            }

            rootLogger.addAppender(candidate);
            root = rootLogger;
            appender = candidate;
        }
    }

    void close() {
        if (appender != null) {
            root.detachAppender(appender);
            appender.stop();
        }
    }

    private static boolean hasSvarogAppender(Logger rootLogger) {
        Iterator<Appender<ILoggingEvent>> appenders = rootLogger.iteratorForAppenders();
        while (appenders.hasNext()) {
            Appender<ILoggingEvent> appender = appenders.next();
            if (appender instanceof SvarogAppender || APPENDER_NAME.equals(appender.getName())) {
                return true;
            }
        }
        return false;
    }
}
