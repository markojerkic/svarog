FROM golang:1.27.3 AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY cmd/client ./cmd/client
COPY internal/lib/util ./internal/lib/util
COPY internal/rpc ./internal/rpc

# Build the client
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '-w -s' -o build/client ./cmd/client

FROM scratch

WORKDIR /svarog

COPY --from=builder /app/build/client /svarog/client
