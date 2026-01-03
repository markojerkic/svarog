FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY ./cmd ./cmd
COPY ./internal ./internal

# Build the server
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0  go build -ldflags '-w -s' -o build/client ./cmd/client

FROM scratch

WORKDIR /svarog

COPY --from=builder /app/build/client /svarog/client
