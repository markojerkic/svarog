package db

import (
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
	Timestamp time.Time
	ID        string
}

type LogServer struct {
	repository LogRepository

	logs chan *StoredLog
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
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	LogLine   string             `bson:"log_line"`
	LogLevel  rpc.LogLevel       `bson:"log_level"`
	Timestamp time.Time          `bson:"timestamp"`
	Client    StoredClient       `bson:"client"`
}

func NewLogServer(dbClient LogRepository) *LogServer {
	return &LogServer{
		logs:       make(chan *StoredLog, 1024*1024),
		repository: dbClient,
	}
}

func (self *LogServer) dumpBacklog(backlog *Backlog) {
	if !backlog.isNotEmpty() {
		return
	}

	self.repository.SaveLogs(backlog.getLogs())
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

func (self *LogServer) runBacklog() {
	backlog := newBacklog()

	go func() {
		for {
			log := <-self.logs
			backlog.add(log)
			if backlog.isFull() {
				self.dumpBacklog(backlog)
			}
		}
	}()
	go func() {
		for {
			<-time.After(5 * time.Second)
			slog.Debug("Timeout reached")
			self.dumpBacklog(backlog)
		}
	}()
}

func (self *LogServer) Run(lines chan *rpc.LogLine) {
	go self.runBacklog()
	for {
		line := <-lines
		if line == nil {
			return
		}

		self.logs <- &StoredLog{
			LogLine:   line.Message,
			LogLevel:  *line.Level.Enum(),
			Timestamp: line.Timestamp.AsTime(),
			Client: StoredClient{
				ClientId:  line.Client,
				IpAddress: "::1",
			},
		}

	}
}
