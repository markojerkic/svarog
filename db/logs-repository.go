package db

import (
	"time"

	rpc "github.com/markojerkic/svarog/proto"
)

type LogRepository interface {
	SaveLogs(logs []interface{}) error
}

type LogServer struct {
	repository LogRepository

	logs chan *StoredLog
}

type StoredLog struct {
	LogLine   string       `bson:"log_line"`
	LogLevel  rpc.LogLevel `bson:"log_level"`
	Timestamp time.Time    `bson:"timestamp"`
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

	for {
		select {
		case log := <-self.logs:
			backlog.add(log)
			if backlog.isFull() {
				self.dumpBacklog(backlog)
			}
		case <-time.After(5 * time.Second):
			self.dumpBacklog(backlog)
		}
	}
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
		}

	}
}
