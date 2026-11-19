package dev.jerkic.svarog.demo.springboot;

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

        if (random.nextInt(100) < OUT_OF_STOCK_PERCENT) {
            LOGGER.warn("Order rejected: out of stock orderId={}", orderId);
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

    private void simulatePayment() throws TimeoutException {
        if (ThreadLocalRandom.current().nextInt(100) < PAYMENT_TIMEOUT_PERCENT) {
            throw new TimeoutException("Simulated payment gateway timeout");
        }
    }
}
