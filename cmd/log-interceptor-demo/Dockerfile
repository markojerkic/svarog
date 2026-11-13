FROM golang:1.27.3-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/log-interceptor-demo ./cmd/log-interceptor-demo
COPY internal/lib/logevent ./internal/lib/logevent
COPY internal/lib/svarogclient ./internal/lib/svarogclient
COPY internal/lib/svaroglog ./internal/lib/svaroglog
COPY internal/rpc ./internal/rpc
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags='-w -s' -o /out/log-interceptor-demo ./cmd/log-interceptor-demo

FROM alpine:3.24.3
WORKDIR /app
COPY --from=builder /out/log-interceptor-demo /usr/local/bin/log-interceptor-demo
USER 1000:1000

# Logging is built into the application; no shell, pipe or Svarog CLI is needed.
ENTRYPOINT ["log-interceptor-demo"]
