package grpcclient

import (
	"context"
	"log"
	"log/slog"
	"sync"
	"time"

	rpc "github.com/markojerkic/svarog/internal/proto"
	"google.golang.org/grpc"
	// "google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
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

	mutex        sync.Mutex
	isConnecting bool
}

var _ Client = &GrpcClient{}

// Run implements Client.
func (self *GrpcClient) Run(ctx context.Context, input <-chan *rpc.LogLine, returnToBacklog ReturnToBacklog) {
	killStreamAndConnection := func() {
		if self.connection != nil {
			self.connection.Close()
		}
		if self.stream != nil {
			self.stream.CloseSend()
		}
		self.connection = nil
		self.stream = nil
		go self.connect()
	}

	killStreamAndConnection()

	for {
		select {
		case <-ctx.Done():
			slog.Debug("Client context done")
			return
		case logLine := <-input:
			if self.stream == nil {
				slog.Debug("Stream is nil, returning to backlog")
				killStreamAndConnection()
				go func() {
					time.Sleep(1 * time.Second)
				}()
				returnToBacklog(logLine)
				continue
			}

			err := self.stream.Send(logLine)
			if err != nil {
				errorCode := status.Code(err)
				slog.Error("Failed to send log line to server", slog.Any("error", err), slog.Any("code", errorCode))
				killStreamAndConnection()
				go func() {
					time.Sleep(1 * time.Second)
					returnToBacklog(logLine)
				}()
			}
		}
	}
}

// BatchSend implements Client.
func (self *GrpcClient) BatchSend(lines []*rpc.LogLine) error {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	_, err := self.client.BatchLog(context.Background(), &rpc.Backlog{
		Logs: lines,
	})
	return err
}

// connect implements Client.
func (self *GrpcClient) connect() error {
	if self.isConnecting {
		return nil
	}

	self.isConnecting = true
	defer func() {
		self.isConnecting = false
	}()
	self.mutex.Lock()
	defer self.mutex.Unlock()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(self.credentials),
		// grpc.WithConnectParams(grpc.ConnectParams{
		// 	Backoff: backoff.Config{
		// 		BaseDelay:  1000,
		// 		Multiplier: 2,
		// 		Jitter:     0,
		// 		MaxDelay:   5000,
		// 	},
		// 	MinConnectTimeout: 2000,
		// }),
	}

	self.connection = nil
	self.stream = nil

	conn, err := grpc.Dial(self.serverAddress, opts...)
	if err != nil {
		slog.Debug("Failed to connect to server", slog.String("address", self.serverAddress), slog.Any("error", err))
		return err
	}

	self.connection = conn
	self.client = rpc.NewLoggAggregatorClient(self.connection)
	stream, err := self.client.Log(context.Background(), grpc.EmptyCallOption{})
	if err != nil {
		log.Fatal("Unable to create stream", err)
		return err
	}
	self.stream = stream
	return nil
}

func NewClient(serverAddress string, credentials credentials.TransportCredentials) Client {
	return &GrpcClient{
		serverAddress: serverAddress,
		credentials:   credentials,
		mutex:         sync.Mutex{},
		isConnecting:  false,
	}
}
