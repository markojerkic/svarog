dependencies:
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
		go install github.com/cosmtrek/air@latest && \
		go install github.com/a-h/templ/cmd/templ@latest

build-proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/schema.proto

build-client-dev:
	go build -o build/client ./cmd/client
build-client:
	go build -ldflags "-s -w" -o build/client ./cmd/client

generate-server:
	templ generate
build-server-dev:
	go build -o build/server ./cmd/server
build-server:
	go build -ldflags "-s -w" -o build/server ./cmd/server
run-server: build-server-dev
	build/server


build-dev: build-server-dev
build: build-client build-server
