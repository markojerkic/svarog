package dev.jerkic.svarog.demo;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public final class Main {
    public static void main(String[] args) {
        Logger logger = LoggerFactory.getLogger(Main.class);
        logger.info("Hello from {}!", "svarog-demo");
    }
}
