# Minimal Go helper

Reads one JSON object per line from stdin and prints its message and original
timestamp to stderr. Stdout stays empty. EOF ends the process successfully;
invalid JSON, missing timestamps and oversized lines stop it with an error.
Input lines are limited to approximately 4 MiB.

From the repository root:

```sh
printf '%s\n' '{"message":"Hello from Java","timestamp":"2026-11-16T10:00:00.123456789Z"}' \
  | go run ./cmd/svarog-bridge
```

Output on stderr:

```text
timestamp=2026-11-16T10:00:00.123456789Z message="Hello from Java"
```

This is only an input/output prototype. It does not connect to NATS, publish to
Svarog or send protocol acknowledgements. Java integration is not implemented yet.
