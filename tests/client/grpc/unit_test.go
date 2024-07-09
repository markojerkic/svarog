package grpc

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
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
}

// BatchLog implements rpc.LoggAggregatorServer.
func (m *MockServer) BatchLog(ctx context.Context, lines *rpc.Backlog) (*rpc.Void, error) {
	receivedLines = append(receivedLines, lines.Logs...)
	log.Printf("Mock server: Received batch log of size: %d", len(lines.Logs))

	return &rpc.Void{}, nil
}

// Log implements rpc.LoggAggregatorServer.
func (m *MockServer) Log(stream rpc.LoggAggregator_LogServer) error {
	for {
		logLine, err := stream.Recv()
		if err != nil {
			return err
		}
		log.Printf("Mock server: Received log line: '%s'", logLine.Message)

		receivedLines = append(receivedLines, logLine)
	}
}

// mustEmbedUnimplementedLoggAggregatorServer implements rpc.LoggAggregatorServer.
func (m *MockServer) mustEmbedUnimplementedLoggAggregatorServer() {}

var _ rpc.LoggAggregatorServer = &MockServer{}

func createMockGrpcServer(serverAddress *string) (*grpc.Server, func() error, string) {
	lis, err := net.Listen("tcp", optional.GetOrDefault(serverAddress, "localhost:0"))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	rpc.RegisterLoggAggregatorServer(grpcServer, &MockServer{})

	addr := lis.Addr().String()

	listen := func() error {
		// Check if listener is active

		log.Printf("Recreated server at address: %s", addr)
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

	return grpcServer, listen, addr
}

var receivedLines []*rpc.LogLine = make([]*rpc.LogLine, 0)

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
	createDebugLogger()
	server, listen, addr := createMockGrpcServer(nil)
	log.Printf("Created server at ddress: %s", addr)
	go listen()
	log.Println("Server started")

	creds := insecure.NewCredentials()
	client := grpcclient.NewClient(addr, creds)
	log.Println("Created client")

	input := make(chan *rpc.LogLine, 10)
	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	go client.Run(ctx, input, func(ll *rpc.LogLine) {
		log.Println("Log line returned because no connection to the server")
		time.Sleep(300 * time.Millisecond)
		input <- ll
	})
	log.Println("Client started")

	for i := 0; i < 10; i++ {
		input <- generateLogLine(i)
		log.Printf("Sent log line %d", i)
	}

    time.Sleep(2 * time.Second)

	server.Stop()
	log.Println("Server stopped")

	for i := 0; i < 10; i++ {
		input <- generateLogLine(i)
	}

	log.Println("Sleeping for 5 seconds before restarting server")
	time.Sleep(5 * time.Second)

	server, listen, addr = createMockGrpcServer(&addr)
	go listen()
	log.Println("Server restarted")

	time.Sleep(8 * time.Second)
	ctx.Done()

	// Assert channel is empty
	assert.Equal(t, 0, len(input), "Expected input channel to be empty")
	close(input)
	server.Stop()

	assert.Equal(t, 20, len(receivedLines), "Expected 20 log lines to be received")
}
