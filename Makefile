PROTOC_GEN_GO_VERSION := v1.36.9
PROTOC_GEN_GO_GRPC_VERSION := v1.5.1
BUF_VERSION := v1.57.2

build-dependencies:
	go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)

deps: build-dependencies
	go mod tidy
	cd web && bun install

build-proto:
	PATH=$(shell go env GOPATH)/bin:$$PATH buf generate internal/proto/schema.proto

build-client-dev:
	go build -o build/client ./cmd/client
build-client:
	go build -ldflags "-s -w" -o build/client ./cmd/client

build-server-dev:
	go build -o build/server ./cmd/server
build-server:
	go build -ldflags "-s -w" -o build/server ./cmd/server
run-server: build-server-dev
	build/server

dev-web:
	@cd web && bun dev
dev-server:
	@watchexec -r -e go -d 1s -- go run cmd/server/main.go

dev:
	@docker compose -f ./docker-compose.dev.yml up -d
	@make -j dev-server dev-web

build-dev: build-server-dev

build-web:
	cd web && bun run build

build: build-client build-server build-web

lint-go:
	gofmt -l .
lint-web:
	cd web && bun lint && bun format:check

format-go:
	go fmt ./...
format-web:
	cd web && bun lint:fix && bun format
format: format-go format-web

test-server:
	go test ./tests/...
test-web:
	cd web && bun run test

test: test-server test-web
test-no-log: test-web
	go test ./tests/...

docker-build-server:
	docker build -t markojerkic/svarog -f ./cmd/server/Dockerfile .
docker-build-client:
	docker build -t markojerkic/svarog-client -f ./cmd/client/Dockerfile .
docker-build: docker-build-server docker-build-client

