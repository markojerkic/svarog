package dev.jerkic.svarog.demo.springboot;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@SpringBootApplication
@RestController
public class SvarogDemoApplication {
    private static final Logger logger = LoggerFactory.getLogger(SvarogDemoApplication.class);

    public static void main(String[] args) {
        SpringApplication.run(SvarogDemoApplication.class, args);
    }

    @GetMapping("/hello")
    public String hello() {
        logger.info("Handled GET /hello in the Svarog Spring Boot demo");
        return "Hello from the Svarog Spring Boot demo!";
    }
}
