package dev.jerkic.svarog.springboot.autoconfigure;

import ch.qos.logback.classic.LoggerContext;
import dev.jerkic.svarog.logback.SvarogAppender;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnBooleanProperty;
import org.springframework.boot.autoconfigure.condition.ConditionalOnClass;
import org.springframework.boot.context.properties.EnableConfigurationProperties;

/** Exposes the settings used by the early logging listener as configuration properties. */
@AutoConfiguration
@ConditionalOnClass({LoggerContext.class, SvarogAppender.class})
@ConditionalOnBooleanProperty(prefix = "svarog.logging", name = "enabled")
@EnableConfigurationProperties(SvarogLoggingProperties.class)
public class SvarogLoggingAutoConfiguration {
}
