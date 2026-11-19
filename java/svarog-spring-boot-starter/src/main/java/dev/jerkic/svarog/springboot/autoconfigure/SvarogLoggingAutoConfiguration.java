package dev.jerkic.svarog.springboot.autoconfigure;

import ch.qos.logback.classic.LoggerContext;
import dev.jerkic.svarog.logback.SvarogAppender;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnBooleanProperty;
import org.springframework.boot.autoconfigure.condition.ConditionalOnClass;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Lazy;

/** Configures Svarog forwarding alongside the application's existing Logback appenders. */
@AutoConfiguration
@ConditionalOnClass({LoggerContext.class, SvarogAppender.class})
@ConditionalOnBooleanProperty(prefix = "svarog.logging", name = "enabled")
@EnableConfigurationProperties(SvarogLoggingProperties.class)
public class SvarogLoggingAutoConfiguration {
    @Bean(initMethod = "start", destroyMethod = "close")
    @Lazy(false)
    public SvarogAppenderRegistration svarogAppenderRegistration(SvarogLoggingProperties properties) {
        return new SvarogAppenderRegistration(properties.bridgePath());
    }
}
