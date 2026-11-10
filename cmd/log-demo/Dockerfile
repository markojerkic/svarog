FROM golang:1.27.3-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/log-demo ./cmd/log-demo
COPY cmd/client ./cmd/client
COPY internal/lib/util ./internal/lib/util
COPY internal/rpc ./internal/rpc
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags='-w -s' -o /out/ ./cmd/log-demo ./cmd/client

FROM alpine:3.24.3
WORKDIR /app
COPY --from=builder /out/log-demo /usr/local/bin/log-demo
COPY --from=builder /out/client /usr/local/bin/svarog-client
COPY cmd/log-demo/messages.txt ./messages.txt
ENV DEMO_MESSAGES_FILE=/app/messages.txt
USER 1000:1000

# A unique instance per process start also works with Linux host networking.
CMD ["sh", "-e", "-o", "pipefail", "-c", "export SVAROG_INSTANCE_ID=\"${SVAROG_INSTANCE_ID:-${DEMO_APP_NAME:-demo}-$(cat /proc/sys/kernel/random/uuid)}\"; log-demo | svarog-client"]
