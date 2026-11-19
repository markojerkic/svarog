package dev.jerkic.svarog.demo;

import ch.qos.logback.classic.LoggerContext;
import org.slf4j.LoggerFactory;

public final class Main {
    public static void main(String[] args) {
        var context = (LoggerContext) LoggerFactory.getILoggerFactory();
        try {
            var logger = LoggerFactory.getLogger(Main.class);
            logger.info("Hello from {}!", "svarog-demo");
        } finally {
            context.stop();
        }
    }
}
