package dev.jerkic.svarog.springboot.autoconfigure;

import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.boot.context.properties.bind.DefaultValue;

/**
 * Spring Boot settings for Svarog forwarding.
 *
 * @param enabled whether to forward Logback events to Svarog
 * @param bridgePath optional executable path, overriding SVAROG_BRIDGE_PATH and the bundled bridge
 */
@ConfigurationProperties("svarog.logging")
public record SvarogLoggingProperties(@DefaultValue("false") boolean enabled, String bridgePath) {
}
