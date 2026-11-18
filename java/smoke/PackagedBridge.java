import ch.qos.logback.classic.LoggerContext;
import ch.qos.logback.core.status.Status;
import ch.qos.logback.core.util.StatusPrinter2;
import dev.jerkic.svarog.logback.SvarogAppender;

class PackagedBridge {
    public static void main(String[] args) {
        LoggerContext context = new LoggerContext();
        SvarogAppender appender = new SvarogAppender();
        appender.setContext(context);
        try {
            appender.start();
            if (!appender.isStarted()) {
                new StatusPrinter2().print(context);
                throw new IllegalStateException("The release JAR did not start its bundled bridge");
            }
        } finally {
            appender.stop();
            context.stop();
        }
        if (context.getStatusManager().getCopyOfStatusList().stream()
            .anyMatch(status -> status.getLevel() == Status.ERROR)) {
            new StatusPrinter2().print(context);
            throw new IllegalStateException("The bundled bridge did not shut down cleanly");
        }
    }
}
