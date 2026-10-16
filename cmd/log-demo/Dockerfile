FROM golang:1.27.2-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -ldflags='-w -s' -o /out/log-demo ./cmd/log-demo
# The existing client's transitive imports include generated UI types.
RUN go tool templ generate ./...
RUN CGO_ENABLED=0 go build -ldflags='-w -s' -o /out/svarog-client ./cmd/client

FROM alpine:3.23.2
WORKDIR /app
COPY --from=builder /out/ /usr/local/bin/
COPY cmd/log-demo/messages.txt ./messages.txt
ENV DEMO_MESSAGES_FILE=/app/messages.txt
USER 1000:1000

# A unique instance per process start also works with Linux host networking.
CMD ["sh", "-e", "-o", "pipefail", "-c", "export SVAROG_INSTANCE_ID=\"${SVAROG_INSTANCE_ID:-${DEMO_APP_NAME:-demo}-$(cat /proc/sys/kernel/random/uuid)}\"; log-demo | svarog-client"]
