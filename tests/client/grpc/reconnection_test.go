package grpc

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	grpcclient "github.com/markojerkic/svarog/cmd/client/grpc-client"
	"github.com/markojerkic/svarog/internal/lib/optional"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MockServer struct {
	rpc.UnimplementedLoggAggregatorServer
	mu sync.Mutex
}

var receivedLines []*rpc.LogLine

func (m *MockServer) BatchLog(ctx context.Context, lines *rpc.Backlog) (*rpc.Void, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	receivedLines = append(receivedLines, lines.Logs...)
	log.Printf("Mock server: Received batch log of size: %d", len(lines.Logs))
	return &rpc.Void{}, nil
}

func (m *MockServer) Log(stream rpc.LoggAggregator_LogServer) error {
	for {
		logLine, err := stream.Recv()
		if err != nil {
			return err
		}
		m.mu.Lock()
		receivedLines = append(receivedLines, logLine)
		log.Printf("Mock server: Received log line: '%s'. New receivedLines size: %d", logLine.Message, len(receivedLines))
		m.mu.Unlock()
	}
}

func createMockGrpcServer(serverAddress *string) (*grpc.Server, func() error, string, *MockServer) {
	lis, err := net.Listen("tcp", optional.GetOrDefault(serverAddress, "localhost:0"))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	mockServer := &MockServer{}
	grpcServer := grpc.NewServer()
	rpc.RegisterLoggAggregatorServer(grpcServer, mockServer)

	addr := lis.Addr().String()
	listen := func() error {
		log.Printf("Server listening at address: %s", addr)
		err := grpcServer.Serve(lis)
		if err != nil {
			lis, err = net.Listen("tcp", addr)
			if err != nil {
				log.Fatalf("Failed to listen: %v", err)
			}
			return grpcServer.Serve(lis)
		}
		return nil
	}
	return grpcServer, listen, addr, mockServer
}

func generateLogLine(index int) *rpc.LogLine {
	return &rpc.LogLine{
		Message:   fmt.Sprintf("Log line %d", index),
		Client:    "marko",
		Timestamp: timestamppb.New(time.Now()),
		Sequence:  int64(index),
	}
}

func createDebugLogger() {
	logOpts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(os.Stdout, logOpts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func TestReconnectingClient(t *testing.T) {
	receivedLines = make([]*rpc.LogLine, 0)
	createDebugLogger()
	server, listen, addr, _ := createMockGrpcServer(nil)
	go listen()
	log.Printf("Server started at address: %s", addr)

	creds := insecure.NewCredentials()
	client := grpcclient.NewClient(addr, creds)
	log.Println("Created client")

	input := make(chan *rpc.LogLine, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go client.Run(ctx, input, func(ll *rpc.LogLine) {
		log.Println("Log line returned to input channel due to connection error")
		time.Sleep(300 * time.Millisecond)
		input <- ll
	})
	log.Println("Client started")

	for i := 0; i < 10; i++ {
		input <- generateLogLine(i)
		log.Printf("Sent log line %d", i)
	}

	time.Sleep(1 * time.Second)

	server.Stop()
	log.Println("Server stopped")

	for i := 10; i < 20; i++ {
		input <- generateLogLine(i)
		log.Printf("Sent log line %d", i)
	}

	log.Println("Sleeping for 5 seconds before restarting server")
	time.Sleep(6 * time.Second)

	server, listen, addr, _ = createMockGrpcServer(&addr)

	go listen()
	log.Println("Server restarted")

	time.Sleep(10 * time.Second)
	cancel()

	server.Stop()
	assert.Equal(t, 20, len(receivedLines), "Expected total of 20 log lines to be received")
}

func TestReconnectingNotStartedClient(t *testing.T) {
	receivedLines = make([]*rpc.LogLine, 0)
	createDebugLogger()
	server, listen, addr, _ := createMockGrpcServer(nil)
	defer server.Stop()

	log.Printf("Server started at address: %s", addr)

	creds := insecure.NewCredentials()
	client := grpcclient.NewClient(addr, creds)
	log.Println("Created client")

	input := make(chan *rpc.LogLine, 30)
	defer close(input)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go client.Run(ctx, input, func(ll *rpc.LogLine) {
		log.Println("Log line returned to input channel due to connection error")
		time.Sleep(300 * time.Millisecond)
		input <- ll
	})
	log.Println("Client started")

	for i := 0; i < 10; i++ {
		input <- generateLogLine(i)
		log.Printf("Sent log line %d", i)
	}

	time.Sleep(1 * time.Second)

	go listen()
	time.Sleep(1 * time.Second)
	server.Stop()
	log.Println("Server stopped")
}
