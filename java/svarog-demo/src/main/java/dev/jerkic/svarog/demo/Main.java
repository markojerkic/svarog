package dev.jerkic.svarog.demo;

import ch.qos.logback.classic.LoggerContext;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public final class Main {
    public static void main(String[] args) {
        LoggerContext context = (LoggerContext) LoggerFactory.getILoggerFactory();
        try {
            Logger logger = LoggerFactory.getLogger(Main.class);
            logger.info("Hello from {}!", "svarog-demo");
        } finally {
            context.stop();
        }
    }
}
