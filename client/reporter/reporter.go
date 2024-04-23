package reporter

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

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
}

type GprcReporter struct {
	serverAddr  string
	credentials credentials.TransportCredentials
	connection  *grpc.ClientConn

	logStream rpc.LoggAggregator_LogClient
	backlog   Backlog[*rpc.LogLine]

	mutex sync.Mutex
}

var _ Reporter = (*GprcReporter)(nil)

// ReportLogLine implements Reporter.
func (self *GprcReporter) ReportLogLine(line *rpc.LogLine) {
	if self.logStream == nil {
		log.Printf("Log stream is nil, adding to backlog\n")
		self.backlog.addToBacklog(line)
		return
	}

	err := self.logStream.Send(line)
	if err != nil {
		log.Printf("Failed to send log line: %v\n", err)
		self.logStream = nil
		go self.createStream()
		self.backlog.addToBacklog(line)
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

func (self *GprcReporter) createStream() {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	client := rpc.NewLoggAggregatorClient(self.connection)

	for {
		if self.logStream != nil {
			break
		}

		if self.connection == nil {
			self.connect()
		}

		stream, err := client.Log(context.Background(), grpc.EmptyCallOption{})
		if err != nil {

			self.logStream = stream
			break
		}

		log.Println("Failed to create a log stream, retrying...")
		time.Sleep(2 * time.Second)
	}
}

func (self *GprcReporter) connect() {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	var opts []grpc.DialOption = []grpc.DialOption{
		grpc.WithTransportCredentials(self.credentials),
	}

	for {
		conn, err := grpc.Dial(self.serverAddr, opts...)

		if err != nil {
			self.connection = conn
			break
		}

		log.Printf("Failed to connect to server %s: %v\n", self.serverAddr, err)
		time.Sleep(5 * time.Second)
	}

	self.createStream()
}

func NewGrpcReporter(serverAddr string, credentials credentials.TransportCredentials) *GprcReporter {
	reporter := &GprcReporter{
		serverAddr:  serverAddr,
		credentials: credentials,
		mutex:       sync.Mutex{},
	}

	reporter.backlog = NewBacklog(func(lines []*rpc.LogLine) error {
		return reporter.ReportBacklogOfLogLines(lines)
	})

	go reporter.connect()

	return reporter
}
