FROM ghcr.io/markojerkic/svarog-client:latest AS svarog

FROM golang:1.27.3-alpine AS builder

WORKDIR /src
COPY main.go ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags='-w -s' -o /out/log-demo main.go

FROM alpine:3.24.3
WORKDIR /app
COPY --from=builder /out/log-demo /usr/local/bin/log-demo
COPY --from=svarog /svarog/client /usr/local/bin/svarog-client
COPY messages.txt ./messages.txt
ENV DEMO_MESSAGES_FILE=/app/messages.txt
USER 1000:1000

# A unique instance per process start also works with Linux host networking.
CMD ["sh", "-e", "-o", "pipefail", "-c", "export SVAROG_INSTANCE_ID=\"${SVAROG_INSTANCE_ID:-${DEMO_APP_NAME:-demo}-$(cat /proc/sys/kernel/random/uuid)}\"; log-demo | svarog-client"]
