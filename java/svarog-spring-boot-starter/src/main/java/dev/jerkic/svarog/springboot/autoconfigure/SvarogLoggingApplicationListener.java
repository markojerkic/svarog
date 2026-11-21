package dev.jerkic.svarog.springboot.autoconfigure;

import java.util.Map;

import org.springframework.beans.factory.support.DefaultSingletonBeanRegistry;
import org.springframework.boot.DefaultPropertiesPropertySource;
import org.springframework.boot.context.event.ApplicationEnvironmentPreparedEvent;
import org.springframework.boot.context.logging.LoggingApplicationListener;
import org.springframework.boot.context.properties.bind.Binder;
import org.springframework.context.ApplicationListener;
import org.springframework.context.ConfigurableApplicationContext;
import org.springframework.context.event.ContextClosedEvent;
import org.springframework.context.event.GenericApplicationListener;
import org.springframework.core.Ordered;
import org.springframework.util.ClassUtils;

/** Attaches the appender after Boot configures Logback, but before it prints the banner. */
public final class SvarogLoggingApplicationListener
    implements ApplicationListener<ApplicationEnvironmentPreparedEvent>, Ordered {
    private static final String REGISTRATION_BEAN = "svarogAppenderRegistration";
    private static final String BANNER_MODE = "spring.main.banner-mode";

    @Override
    public int getOrder() {
        return LoggingApplicationListener.DEFAULT_ORDER + 1;
    }

    @Override
    public void onApplicationEvent(ApplicationEnvironmentPreparedEvent event) {
        var environment = event.getEnvironment();
        if (!Boolean.parseBoolean(environment.getProperty("svarog.logging.enabled"))) {
            return;
        }
        var classLoader = event.getSpringApplication().getClassLoader();
        if (!ClassUtils.isPresent("ch.qos.logback.classic.LoggerContext", classLoader)) {
            return;
        }

        if (!environment.containsProperty(BANNER_MODE)) {
            DefaultPropertiesPropertySource.addOrMerge(Map.of(BANNER_MODE, "log"), environment.getPropertySources());
        }
        var bridgePath = Binder.get(environment).bind("svarog.logging.bridge-path", String.class).orElse(null);
        var registration = new SvarogAppenderRegistration(bridgePath);
        registration.start();
        event.getBootstrapContext().addCloseListener(bootstrap -> registerWithContext(
            bootstrap.getApplicationContext(), registration));
    }

    private void registerWithContext(ConfigurableApplicationContext context, SvarogAppenderRegistration registration) {
        var factory = context.getBeanFactory();
        factory.registerSingleton(REGISTRATION_BEAN, registration);
        if (factory instanceof DefaultSingletonBeanRegistry registry) {
            registry.registerDisposableBean(REGISTRATION_BEAN, () -> {
                // A failed refresh destroys beans before Boot reports the failure. Leave
                // Logback to drain the appender at JVM exit if no normal close follows.
                if (context.isClosed()) {
                    registration.close();
                }
            });
        } else {
            context.addApplicationListener(GenericApplicationListener.forEventType(ContextClosedEvent.class, closed -> {
                if (closed.getApplicationContext() == context) {
                    registration.close();
                }
            }));
        }
    }
}
