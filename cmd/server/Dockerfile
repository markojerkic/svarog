####################################################################################
# Stage 1: Build assets and the server with the same task as a local build
####################################################################################

FROM golang:1.27.3 AS builder

COPY --from=oven/bun:1.4.3 /usr/local/bin/bun /usr/local/bin/bun
RUN go install github.com/go-task/task/v3/cmd/task@v3.46.4

WORKDIR /app

COPY go.mod go.sum package.json bun.lock ./
RUN go mod download
RUN bun install --frozen-lockfile

COPY . .
RUN task build OUT=/app/build/server

####################################################################################
# Stage 2: Copy the binary and the assets it serves
####################################################################################

FROM alpine:3.24.3

WORKDIR /app

COPY --from=builder /app/build/server /app/build/server
COPY --from=builder /app/internal/server/ui/assets/ /app/internal/server/ui/assets/

# Named volumes inherit this directory's ownership on first use.
RUN mkdir -p /var/lib/svarog/archives && chown 1000:1000 /var/lib/svarog/archives

ENV DOCKER=true
ENV HTTP_SERVER_PORT=1323

EXPOSE 1323

USER 1000

CMD ["/app/build/server"]
