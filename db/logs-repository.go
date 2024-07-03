package db

import (
	"context"
	"log/slog"
	"time"

	rpc "github.com/markojerkic/svarog/proto"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LogRepository interface {
	SaveLogs(logs []interface{}) error
	GetLogs(clientId string, cursor *LastCursor) ([]StoredLog, error)
	GetClients() ([]Client, error)
}

type LastCursor struct {
	Timestamp  time.Time
	ID         string
	IsBackward bool
}

type LogServer struct {
	ctx        context.Context
	repository LogRepository

	logs    chan *StoredLog
	backlog *Backlog
}

type Client struct {
	Client   StoredClient
	IsOnline bool
}

type StoredClient struct {
	ClientId  string `bson:"client_id"`
	IpAddress string `bson:"ip_address"`
}

type StoredLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	LogLine        string             `bson:"log_line"`
	LogLevel       rpc.LogLevel       `bson:"log_level"`
	Timestamp      time.Time          `bson:"timestamp"`
	Client         StoredClient       `bson:"client"`
	SequenceNumber int64              `bson:"sequence_number"`
}

func NewLogServer(ctx context.Context, dbClient LogRepository) *LogServer {
	return &LogServer{
		ctx:        ctx,
		repository: dbClient,
		logs:       make(chan *StoredLog, 1024*1024),
		backlog:    newBacklog(),
	}
}

func (self *LogServer) dumpBacklog() {
	if !self.backlog.isNotEmpty() {
		return
	}

	self.repository.SaveLogs(self.backlog.getLogs())
}

var backlogLimit = 1000

type Backlog struct {
	logs  []interface{}
	index int
}

func (self *Backlog) getLogs() []interface{} {
	logs := self.logs[:self.index]

	self.index = 0

	return logs
}

func (self *Backlog) add(log interface{}) {
	self.logs[self.index] = log
	self.index = (self.index + 1) % backlogLimit
}

func (self *Backlog) isNotEmpty() bool {
	return self.index > 0
}

func (self *Backlog) isFull() bool {
	return self.index == backlogLimit-1
}

func newBacklog() *Backlog {
	return &Backlog{
		logs:  make([]interface{}, backlogLimit),
		index: 0,
	}
}

func (self *LogServer) Run(lines chan *rpc.LogLine) {
	for {
		select {
		case line := <-lines:
			logLine := &StoredLog{
				LogLine:        line.Message,
				LogLevel:       *line.Level.Enum(),
				Timestamp:      line.Timestamp.AsTime(),
				SequenceNumber: line.Sequence,
				Client: StoredClient{
					ClientId:  line.Client,
					IpAddress: "::1",
				},
			}
			self.backlog.add(logLine)

			if self.backlog.isFull() {
				self.dumpBacklog()
			}

		case <-time.After(5 * time.Second):
			slog.Debug("Dumping backlog after timeout")
			self.dumpBacklog()

		case <-self.ctx.Done():
			slog.Debug("Context done")
			return
		}
	}
}
