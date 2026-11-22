package dev.jerkic.svarog.demo.springboot;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.net.ConnectException;
import java.util.UUID;
import java.util.concurrent.ThreadLocalRandom;
import java.util.concurrent.TimeoutException;

import net.datafaker.Faker;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

@Component
public class OrderLogSimulator {
    private static final Logger LOGGER = LoggerFactory.getLogger(OrderLogSimulator.class);
    private static final int OUT_OF_STOCK_PERCENT = 10;
    private static final int INVENTORY_OFFLINE_PERCENT = 5;
    private static final int INVALID_STOCK_LEVEL_PERCENT = 5;
    private static final int PAYMENT_TIMEOUT_PERCENT = 15;
    private static final int MAX_PAYMENT_ATTEMPTS = 2;

    private final Faker faker = new Faker();

    @Scheduled(fixedRateString = "${demo.interval:1s}")
    public void simulateOrder() {
        var random = ThreadLocalRandom.current();
        var orderId = UUID.randomUUID();
        var product = faker.commerce().productName();
        var quantity = random.nextInt(1, 6);
        LOGGER.info("Order received orderId={} product=\"{}\" quantity={}", orderId, product, quantity);

        try {
            if (availableStock(product) < quantity) {
                LOGGER.warn("Order rejected: out of stock orderId={}", orderId);
                return;
            }
        } catch (UncheckedIOException error) {
            LOGGER.error("Order failed: inventory lookup orderId={} product=\"{}\"", orderId, product, error);
            return;
        }

        for (int attempt = 1; attempt <= MAX_PAYMENT_ATTEMPTS; attempt++) {
            try {
                simulatePayment();
                LOGGER.info("Order completed orderId={} paymentAttempts={}", orderId, attempt);
                return;
            } catch (TimeoutException error) {
                if (attempt < MAX_PAYMENT_ATTEMPTS) {
                    LOGGER.warn("Payment timed out, retrying orderId={} attempt={}", orderId, attempt);
                } else {
                    LOGGER.error("Order failed: payment timed out orderId={} attempts={}", orderId, attempt, error);
                }
            }
        }
    }

    private int availableStock(String product) {
        try {
            return fetchStockLevel();
        } catch (IOException error) {
            throw new UncheckedIOException("Could not check availability of " + product, error);
        }
    }

    private int fetchStockLevel() throws IOException {
        try {
            return Integer.parseInt(readInventoryResponse());
        } catch (NumberFormatException error) {
            throw new IOException("Inventory returned a non-numeric stock level", error);
        } catch (IOException error) {
            throw new IOException("No stock level received from inventory", error);
        }
    }

    private String readInventoryResponse() throws IOException {
        var random = ThreadLocalRandom.current();
        var outcome = random.nextInt(100);
        if (outcome < INVENTORY_OFFLINE_PERCENT) {
            throw new IOException("Could not open inventory connection", new ConnectException("Connection refused"));
        }
        if (outcome < INVENTORY_OFFLINE_PERCENT + INVALID_STOCK_LEVEL_PERCENT) {
            return "unknown";
        }
        return random.nextInt(100) < OUT_OF_STOCK_PERCENT ? "0" : "25";
    }

    private void simulatePayment() throws TimeoutException {
        if (ThreadLocalRandom.current().nextInt(100) < PAYMENT_TIMEOUT_PERCENT) {
            throw new TimeoutException("Simulated payment gateway timeout");
        }
    }
}
