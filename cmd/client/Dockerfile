FROM golang:1.27.3 AS builder

WORKDIR /app

COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY ./cmd ./cmd
COPY ./internal ./internal

# Generate UI types required by the client's transitive imports.
RUN go tool templ generate ./...

# Build the client
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0  go build -ldflags '-w -s' -o build/client ./cmd/client

FROM scratch

WORKDIR /svarog

COPY --from=builder /app/build/client /svarog/client
