package dev.jerkic.svarog.logback;

import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonGenerator;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.OutputStream;
import java.time.Instant;

final class BridgeFrame extends OutputStream {
    // cmd/svarog-bridge's stdin scanner limit, including the line terminator.
    static final int MAX_BYTES = 4 * 1024 * 1024;
    private static final Instant MIN_TIMESTAMP = Instant.parse("0000-01-01T00:00:00Z");
    private static final Instant GO_ZERO_TIMESTAMP = Instant.parse("0001-01-01T00:00:00Z");
    private static final Instant MAX_TIMESTAMP_EXCLUSIVE = Instant.parse("+10000-01-01T00:00:00Z");
    static final JsonFactory JSON = new JsonFactory();

    private final ByteArrayOutputStream bytes = new ByteArrayOutputStream();

    static byte[] encode(String message, Instant timestamp) throws IOException {
        if (message.indexOf('\0') >= 0 || timestamp == null || timestamp.isBefore(MIN_TIMESTAMP)
            || !timestamp.isBefore(MAX_TIMESTAMP_EXCLUSIVE) || timestamp.equals(GO_ZERO_TIMESTAMP)) {
            throw new IOException("Invalid Svarog bridge record");
        }
        BridgeFrame frame = new BridgeFrame();
        try (JsonGenerator output = JSON.createGenerator(frame)) {
            output.disable(JsonGenerator.Feature.AUTO_CLOSE_JSON_CONTENT);
            output.writeStartObject();
            output.writeStringField("message", message);
            output.writeStringField("timestamp", timestamp.toString());
            output.writeEndObject();
            output.writeRaw('\n');
        }
        return frame.bytes.toByteArray();
    }

    @Override
    public void write(int value) throws IOException {
        checkSize(1);
        bytes.write(value);
    }

    @Override
    public void write(byte[] source, int offset, int length) throws IOException {
        checkSize(length);
        bytes.write(source, offset, length);
    }

    private void checkSize(int length) throws IOException {
        if (length > MAX_BYTES - bytes.size()) {
            throw new IOException("Svarog bridge record exceeds the input limit");
        }
    }
}
