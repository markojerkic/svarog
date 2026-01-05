# Svarog

A modern log aggregation and monitoring system that collects logs from distributed clients via NATS messaging and provides a web interface for viewing and analyzing them in real-time.

## Architecture

Svarog uses a NATS-based pub/sub architecture for reliable log ingestion:

- **Clients** pipe their application logs to the Svarog client, which publishes them to a NATS server
- **NATS** acts as the message broker, ensuring reliable delivery and decoupling clients from the server
- **Server** subscribes to NATS topics, aggregates logs in MongoDB, and serves them via HTTP/WebSocket
- **Web UI** provides real-time log viewing with filtering, search, and project management

### Connection Model

Clients connect using a generated connection URL that contains all necessary configuration:

```
svarog://nats-server:port/logs.project.client?token=authentication_token
```

This URL format provides:

- **NATS server address** (`nats-server:port`)
- **Topic hierarchy** (`logs.project.client`) for organizing logs by project and client
- **Authentication token** for secure access

The server generates these connection URLs through the web interface, making it easy to integrate new clients with a simple copy/paste workflow.

## Development

Start the development environment with hot reload:

```bash
task db                    # Start MongoDB in Docker
task dev                   # Start server with templ hot reload
```

## Client Usage

The Svarog client reads logs from stdin and publishes them to NATS. Integrate it into your application by piping logs:

### Docker Integration

```Dockerfile
FROM svarog-client:latest AS svarog-client

FROM alpine:3.12

COPY ./your-app .
COPY --from=svarog-client /svarog/client /svarog/client

# Pipe application output to Svarog client
CMD ["sh", "-c", "./your-app | /svarog/client"]
```

### Configuration

Configure the client using the connection URL generated from the Svarog web interface. The URL can be provided either as a command-line argument or via the `SVAROG_CONN_STRING` environment variable.

**Via environment variable:**

```yaml
# docker-compose.yml
version: "3"
services:
  your-app:
    image: your-app:latest
    environment:
      - SVAROG_CONN_STRING=svarog://nats-server:4222/logs.myproject.myapp?token=base64_encoded_token
      - SVAROG_INSTANCE_ID=app-instance-01 # Optional: defaults to hostname, which in docker is the container name
```

**Via command-line argument:**

```bash
your-app | svarog-client "svarog://nats-server:4222/logs.myproject.myapp?token=base64_token"
```

**Optional query parameters:**

- `debug=true` - Enable debug logging from the client

**Environment variables:**

- `SVAROG_CONN_STRING` - The complete connection URL (if not provided as argument)
- `SVAROG_INSTANCE_ID` - Custom instance identifier (defaults to hostname)

### Command Line Usage

```bash
# Using environment variable
export SVAROG_CONN_STRING="svarog://nats:4222/logs.proj.app?token=xyz"
tail -f /var/log/app.log | svarog-client

# Using command-line argument
tail -f /var/log/app.log | svarog-client "svarog://nats:4222/logs.proj.app?token=xyz"

# With debug enabled in URL
tail -f /var/log/app.log | svarog-client "svarog://nats:4222/logs.proj.app?token=xyz&debug=true"
```

## Server Deployment

Deploy the complete Svarog stack with NATS, MongoDB, and the web server:

```yaml
# docker-compose.yml
version: "3"
services:
  svarog-nats:
    image: nats:latest
    container_name: svarog-nats
    ports:
      - 4222:4222
      - 8222:8222 # HTTP monitoring
    command: ["-js"] # Enable JetStream for persistence

  svarog-server:
    image: svarog:latest
    container_name: svarog-server
    ports:
      - 1323:1323 # HTTP/WebSocket interface
    environment:
      - MONGO_URL=mongodb://user:pass@svarog-mongodb:27017/
      - NATS_URL=nats://svarog-nats:4222
      - HTTP_SERVER_PORT=1323
      - HTTP_SERVER_ALLOWED_ORIGINS=http://localhost:1323
    depends_on:
      - svarog-mongodb
      - svarog-nats

  svarog-mongodb:
    image: mongodb/mongodb-community-server:6.0-ubi8
    container_name: svarog-mongodb
    ports:
      - 27017:27017
    environment:
      - MONGODB_INITDB_ROOT_USERNAME=user
      - MONGODB_INITDB_ROOT_PASSWORD=pass
    volumes:
      - dbdata:/data/db

volumes:
  dbdata:
```

## Getting Started

1. **Deploy the server stack** using the docker-compose configuration above
2. **Access the web interface** at http://localhost:1323
3. **Create a project** for organizing your logs
4. **Generate a connection URL** for your client application
5. **Integrate the client** into your application using the provided URL
6. **View logs** in real-time through the web interface

## Features

- Real-time log streaming via WebSocket
- Project-based log organization
- Full-text search and filtering
- Log archiving and retention policies
- Authentication and access control
- Responsive web interface with HTMX
