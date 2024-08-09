package grpcclient

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	rpc "github.com/markojerkic/svarog/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type ReturnToBacklog func(*rpc.LogLine)
type Client interface {
	Run(context.Context, <-chan *rpc.LogLine, ReturnToBacklog)
	BatchSend([]*rpc.LogLine) error
	connect() error
}

type GrpcClient struct {
	serverAddress string
	credentials   credentials.TransportCredentials
	connection    *grpc.ClientConn
	stream        rpc.LoggAggregator_LogClient
	client        rpc.LoggAggregatorClient

	mutex sync.Mutex
}

var _ Client = &GrpcClient{}

// connect establishes a connection and creates a new stream
func (g *GrpcClient) connect() error {
	var err error
	g.connection, err = grpc.Dial(g.serverAddress, grpc.WithTransportCredentials(g.credentials))
	if err != nil {
		return err
	}
	g.client = rpc.NewLoggAggregatorClient(g.connection)
	g.stream, err = g.client.Log(context.Background())
	return err
}

// Run listens on the channel and sends messages to the gRPC stream
func (g *GrpcClient) Run(ctx context.Context, ch <-chan *rpc.LogLine, returnToBacklog ReturnToBacklog) {
	for {
		select {
		case <-ctx.Done():
			slog.Debug("Client context done")
			return
		case logLine := <-ch:
			if err := g.sendLogLine(logLine); err != nil {
				slog.Debug("Sending log line failed", slog.Any("error", err))
				returnToBacklog(logLine)
				g.reconnect()
			}
		}
	}
}

// sendLogLine sends a single log line to the gRPC stream
func (g *GrpcClient) sendLogLine(logLine *rpc.LogLine) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if g.stream == nil {
		return fmt.Errorf("gRPC stream is not initialized")
	}

	if err := g.stream.Send(logLine); err != nil {
		return fmt.Errorf("Failed to send log line: %v", err)
	}

	return nil
}

// BatchSend sends a batch of log lines to the gRPC stream
func (g *GrpcClient) BatchSend(logLines []*rpc.LogLine) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for _, logLine := range logLines {
		if err := g.stream.Send(logLine); err != nil {
			return fmt.Errorf("Failed to send log line: %v", err)
		}
	}

	return nil
}

// reconnect attempts to re-establish the gRPC connection and stream
func (g *GrpcClient) reconnect() {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for {
		slog.Debug("Attempting to reconnect...", slog.String("server address", g.serverAddress))
		if err := g.connect(); err == nil {
			slog.Debug("Reconnected successfully")
			return
		}
		slog.Debug("Failed to reconnect, retrying in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func NewClient(serverAddress string, credentials credentials.TransportCredentials) Client {
	return &GrpcClient{
		serverAddress: serverAddress,
		credentials:   credentials,
		mutex:         sync.Mutex{},
	}
}
