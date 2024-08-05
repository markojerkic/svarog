# ####################################################################################
# # Stage 1: Build the web assets
# ####################################################################################
#
# FROM oven/bun:1 AS base-web
# WORKDIR /usr/src/app
#
# # install dependencies into temp directory
# # this will cache them and speed up future builds
# FROM base-web AS install
# RUN mkdir -p /temp/dev
# COPY ./web/package.json ./web/bun.lockb /temp/dev/
# RUN cd /temp/dev && bun install --frozen-lockfile
#
# # install with --production (exclude devDependencies)
# RUN mkdir -p /temp/prod
# COPY ./web/package.json ./web/bun.lockb /temp/prod/
# RUN cd /temp/prod && bun install --frozen-lockfile --production
#
# # copy node_modules from temp directory
# # then copy all (non-ignored) project files into the image
# FROM base-web AS web-build
# COPY --from=install /temp/dev/node_modules node_modules
# COPY ./web .
#
# # [optional] tests & build
# ENV NODE_ENV=production
# RUN bun run build
#
####################################################################################
# Stage 2: Build the Go app
####################################################################################

# Stage 1: Build the Go server
FROM golang:1.20 AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the server
RUN go build -o build/server ./cmd/server

# Stage 3: Create the final image
FROM scratch

# Set the working directory
WORKDIR /app

# Copy the server binary and web assets from the previous stages
COPY --from=builder /app/build/server /app/server
# COPY --from=web-build /usr/src/app/dist /app/web/dist

# Expose the port the server will run on
EXPOSE 1323

# Command to run the server
CMD ["/app/server"]

