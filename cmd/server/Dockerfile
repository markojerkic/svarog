####################################################################################
# Stage 1: Build the web assets
####################################################################################

FROM oven/bun:1 AS base-web
WORKDIR /usr/src/app

# install dependencies into temp directory
# this will cache them and speed up future builds
FROM base-web AS install
RUN mkdir -p /temp/dev
COPY ./web/package.json ./web/bun.lockb /temp/dev/
RUN cd /temp/dev && bun install --frozen-lockfile

# install with --production (exclude devDependencies)
RUN mkdir -p /temp/prod
COPY ./web/package.json ./web/bun.lockb /temp/prod/
RUN cd /temp/prod && bun install --frozen-lockfile --production

# copy node_modules from temp directory
# then copy all (non-ignored) project files into the image
FROM base-web AS web-build
COPY --from=install /temp/dev/node_modules node_modules
COPY ./web .

# [optional] tests & build
ENV NODE_ENV=production
RUN bun run build

####################################################################################
# Stage 2: Build the Go app
####################################################################################

FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY ./cmd ./cmd
COPY ./internal ./internal

# Build the server
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0  go build -ldflags '-w -s' -o build/server ./cmd/server

####################################################################################
# Stage 3: Copy all needed files and run
####################################################################################

FROM alpine:3.21.2

# Set the working directory
WORKDIR /app

# Copy the server binary and web assets from the previous stages
COPY --from=builder /app/build/server /app/build/server
COPY --from=web-build /usr/src/app/dist /app/public

ENV DOCKER=true

# Expose the port the server will run on
EXPOSE 1323
EXPOSE 50051

# Command to run the server
USER 1000

CMD ["/app/build/server"]
