build-dependencies:
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/cosmtrek/air@latest

deps: build-dependencies
	go mod tidy
	cd web && bun install

build-proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative internal/proto/schema.proto

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


watch:
	@docker compose -f ./docker-compose.dev.yml up -d
	@watchexec -r -e go -d 1s -- go run cmd/server/main.go
	# @go run github.com/cosmtrek/air@v1.51.0

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
	go test ./tests/... -v
test-web:
	cd web && bun run test

test: test-server test-web
test-no-log: test-web
	go test ./tests/...

docker-build-server:
	docker build -t svarog -f ./cmd/server/Dockerfile .
docker-build-client:
	docker build -t svarog-client -f ./cmd/client/Dockerfile .
