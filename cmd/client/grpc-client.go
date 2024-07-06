package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	rpc "github.com/markojerkic/svarog/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Client interface {
	Run(context.Context, <-chan *rpc.LogLine, chan<- *rpc.LogLine)
	connect()
}

type GrpcClient struct {
	serverAddress string
	credentials   credentials.TransportCredentials
	connection    *grpc.ClientConn
	stream        rpc.LoggAggregator_LogClient

	mutex sync.Mutex
}

var _ Client = &GrpcClient{}

// Run implements Client.
func (self *GrpcClient) Run(ctx context.Context, input <-chan *rpc.LogLine, returnToBacklog chan<- *rpc.LogLine) {
	go self.connect()
	for {
		select {
		case <-ctx.Done():
			slog.Debug("Client context done")
			return
		case logLine := <-input:
			err := self.stream.Send(logLine)
			if err != nil {
				slog.Debug("Failed to send log line to server", slog.Any("error", err))
				returnToBacklog <- logLine
				go self.connect()
			}
		}

	}
}

// connect implements Client.
func (self *GrpcClient) connect() {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(self.credentials),
	}

	// If connection is ok and stream is ok, then return
	if self.connection != nil && self.stream != nil {
		return
	}

	for {
		if self.connection != nil {
			self.connection.Close()
		}
		if self.stream != nil {
			go self.stream.CloseAndRecv()
		}

		conn, err := grpc.Dial(self.serverAddress, opts...)
		slog.Debug("Connecting to server")

		if err != nil {
			slog.Debug("Failed to connect to server", slog.String("address", self.serverAddress), slog.Any("error", err))
			time.Sleep(5 * time.Second)

			continue
		}
		self.connection = conn

		client := rpc.NewLoggAggregatorClient(self.connection)
		stream, err := client.Log(context.Background(), grpc.EmptyCallOption{})
		if err != nil {
			slog.Debug("Failed to open stream to server", slog.Any("error", err))
			continue
		}
		self.stream = stream

	}
}

func NewClient(serverAddress string, credentials credentials.TransportCredentials) Client {
	return &GrpcClient{
		serverAddress: serverAddress,
		credentials:   credentials,
	}
}
