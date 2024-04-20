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

func (self *LogServer) dumpBacklog(backlog []interface{}) {
	if len(backlog) == 0 {
		return
	}

	self.repository.SaveLogs(backlog)
}

var backlogLimit = 1000

func (self *LogServer) runBacklog() {
	backlog := make([]interface{}, 0, backlogLimit)

	for {
		select {
		case log := <-self.logs:
			backlog = append(backlog, log)
			if len(backlog) == backlogLimit {
				self.dumpBacklog(backlog)
				backlog = backlog[:0]
			}
		case <-time.After(5 * time.Second):
			self.dumpBacklog(backlog)
			backlog = backlog[:0]
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
