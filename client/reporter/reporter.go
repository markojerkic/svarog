package reporter

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/labstack/gommon/log"
	rpc "github.com/markojerkic/svarog/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Reporter interface {
	// ReportLogLine sends a log line to the server.
	// If error occurs, it will be added to the backlog.
	ReportLogLine(line *rpc.LogLine)
	// Used to send the backlog to the server.
	ReportBacklogOfLogLines(lines []*rpc.LogLine) error
	IsSafeToClose() bool
}

type GprcReporter struct {
	serverAddr  string
	credentials credentials.TransportCredentials
	connection  *grpc.ClientConn

	logStream rpc.LoggAggregator_LogClient
	backlog   Backlog[*rpc.LogLine]

	createStreamMutex   sync.Mutex
	openConnectionMutex sync.Mutex
	isSafeToClose       bool
}

// IsSafeToClose implements Reporter.
func (self *GprcReporter) IsSafeToClose() bool {
	return self.isSafeToClose
}

var _ Reporter = (*GprcReporter)(nil)

func (self *GprcReporter) createStream() {
	self.createStreamMutex.Lock()
	defer func() {
		slog.Debug("Releasing createStreamMutex")
		self.createStreamMutex.Unlock()
	}()

	client := rpc.NewLoggAggregatorClient(self.connection)

	for {
		if self.logStream != nil {
            slog.Debug("Log stream already exists")
			break
		}

		if self.connection == nil {
            slog.Debug("Connection is nil, connecting...")
			self.connect()
		}

		stream, err := client.Log(context.Background(), grpc.EmptyCallOption{})
		if err == nil {
			self.logStream = stream
			break
		}

		slog.Debug("Failed to create a log stream, retrying...")
		time.Sleep(2 * time.Second)
	}

    slog.Debug("Log stream created")
}

func (self *GprcReporter) connect() {
	self.openConnectionMutex.Lock()
	defer func() {
		slog.Debug("Releasing openConnectionMutex")
		self.openConnectionMutex.Unlock()
	}()

	if self.connection != nil {
		return
	}

	var opts []grpc.DialOption = []grpc.DialOption{
		grpc.WithTransportCredentials(self.credentials),
	}

	for {
		conn, err := grpc.Dial(self.serverAddr, opts...)
		slog.Debug("Connecting to server")

		if err == nil {
			self.connection = conn
			break
		}

		slog.Debug("Failed to connect to server %s: %v\n", self.serverAddr, err)
		time.Sleep(5 * time.Second)
	}

	slog.Debug("Connected to server")

	self.createStream()
}

// ReportLogLine implements Reporter.
func (self *GprcReporter) ReportLogLine(line *rpc.LogLine) {

    slog.Debug("Reporting log line")

	if self.logStream == nil {
		log.Debug("Log stream is nil, adding to backlog")
		self.backlog.addToBacklog(line)
		go self.createStream()
		return
	}

    slog.Debug("Sending log line")
	err := self.logStream.Send(line)
	if err != nil {
		slog.Debug("Failed to send log line: %v\n", err)
		self.logStream = nil
		go self.createStream()
		self.backlog.addToBacklog(line)
	} else {
        slog.Debug("Log line sent")
    }
}

// ReportBacklogOfLogLines implements Reporter.
func (self *GprcReporter) ReportBacklogOfLogLines(lines []*rpc.LogLine) error {

	if self.connection == nil {
		go self.connect()
		return errors.New("Connection is not established")
	}

	client := rpc.NewLoggAggregatorClient(self.connection)

	_, err := client.BatchLog(context.Background(), &rpc.Backlog{
		Logs: lines,
	})

	return err
}

func NewGrpcReporter(serverAddr string, credentials credentials.TransportCredentials) *GprcReporter {
	reporter := &GprcReporter{
		serverAddr:  serverAddr,
		credentials: credentials,

		openConnectionMutex: sync.Mutex{},
		createStreamMutex:   sync.Mutex{},
		isSafeToClose:       false,
	}

	go reporter.connect()

	reporter.backlog = NewBacklog(func(lines []*rpc.LogLine) error {
		return reporter.ReportBacklogOfLogLines(lines)
	})

	return reporter
}
