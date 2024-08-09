# Client usage

```Dockerfile
FROM svarog-client:latest AS svarog-client

FROM alpine:3.12

COPY ./echo.sh .
COPY --from=svarog-client /svarog /svarog/

CMD ["sh", "-c", "sh echo.sh | /svarog/client -SVAROG_DEBUG_ENABLED -SVAROG_CLIENT_ID=$SVAROG_CLIENT_ID -SVAROG_SERVER_ADDR=$SVAROG_SERVER_ADDR"]
# or
CMD ["sh", "-c", "sh echo.sh | /svarog/client"] # and set the environment variables in the docker-compose.yml
```

```yaml docker-compose.yml
version: '3'
services:
  svarog-echo-example:
    image: svarog-echo-example:latest
    container_name: svarog-echo-example
    build:
      context: .
    extra_hosts:
      - "host.docker.internal:host-gateway"
    environment:
      - SVAROG_SERVER_ADDR=host.docker.internal:50051
      - SVAROG_CLIENT_ID=svarog-echo
      - SVAROG_DEBUG_ENABLED=true
```

# Server usage

```yaml docker-compose.yml
version: '3'
services:
  svarog:
    image: svarog:latest
    container_name: svarog
    ports:
      - 1323:1323
      - 50051:50051
    environment:
    - MONGO_URL=mongodb://user:pass@svarog-mongodb:27017/
    - GPRC_PORT=50051
    - HTTP_SERVER_PORT=1323
    - HTTP_SERVER_ALLOWED_ORIGINS=http://localhost:3000
    depends_on:
      - svarog-mongodb
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
