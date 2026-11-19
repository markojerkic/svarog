FROM golang:1.27.3 AS build-assets

COPY --from=oven/bun:1.4.3 /usr/local/bin/bun /usr/local/bin/bun
RUN go install github.com/go-task/task/v3/cmd/task@v3.46.4

WORKDIR /app

COPY go.mod go.sum package.json bun.lock ./
RUN go mod download
RUN bun install --frozen-lockfile

COPY . .
RUN task build:assets

FROM build-assets AS build-server
RUN task build:binary CMD=server OUT=/app/build/server

FROM build-assets AS build-server-embedded
RUN task build:binary CMD=server-embedded OUT=/app/build/server-embedded

FROM alpine:3.24.3 AS runtime

WORKDIR /app
COPY --from=build-assets /app/internal/server/ui/assets/ /app/internal/server/ui/assets/

# Named volumes inherit this directory's ownership on first use.
RUN mkdir -p /var/lib/svarog/archives && chown 1000:1000 /var/lib/svarog/archives

ENV DOCKER=true
ENV HTTP_SERVER_PORT=1323

EXPOSE 1323

FROM runtime AS server-embedded
COPY --from=build-server-embedded /app/build/server-embedded /app/build/server-embedded
RUN mkdir -p /var/lib/svarog/nats && chown 1000:1000 /var/lib/svarog/nats
ENV NATS_STORE_DIR=/var/lib/svarog/nats
EXPOSE 4222
USER 1000
CMD ["/app/build/server-embedded"]

# Keep the external-NATS image as the default target.
FROM runtime AS server
COPY --from=build-server /app/build/server /app/build/server
USER 1000
CMD ["/app/build/server"]
