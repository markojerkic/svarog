dependencies:
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

build-proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/schema.proto

run-server:
	go run cmd/main.go

build-client:
	go build -o build/client client/client.go
build-server:
	go build -o build/server cmd/main.go

build: build-client build-server
