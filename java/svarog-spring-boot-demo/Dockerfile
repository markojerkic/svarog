FROM maven:3.9-eclipse-temurin-17 AS builder

WORKDIR /src
COPY pom.xml ./
COPY src ./src
RUN --mount=type=cache,target=/root/.m2/repository \
    mvn --batch-mode --no-transfer-progress package

FROM eclipse-temurin:17-jre

WORKDIR /app
COPY --from=builder /src/target/svarog-spring-boot-demo-*.jar /app/svarog-demo.jar
USER 1000:1000

# Replicas on the host network share a hostname.
ENTRYPOINT ["sh", "-ec", "export SVAROG_INSTANCE_ID=\"${SVAROG_INSTANCE_ID:-demo-spring-boot-$(cat /proc/sys/kernel/random/uuid)}\"; exec java -jar /app/svarog-demo.jar \"$@\"", "--"]
