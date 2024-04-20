package db

import (
	"context"
	"fmt"
	"time"

	rpc "github.com/markojerkic/svarog/proto"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LogServer struct {
	mongoClient   *mongo.Client
	logCollection *mongo.Collection

	logs chan *StoredLog
}

type StoredLog struct {
	LogLine   string       `bson:"log_line"`
	LogLevel  rpc.LogLevel `bson:"log_level"`
	Timestamp time.Time    `bson:"timestamp"`
}

func NewLogServer() *LogServer {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://user:pass@localhost:27017"))

	if err != nil {
		panic(err)
	}
	collection := client.Database("logs").Collection("log_lines")

	return &LogServer{
		mongoClient:   client,
		logCollection: collection,
		logs:          make(chan *StoredLog, 1024*1024),
	}
}

func (self *LogServer) dumpBacklog(backlog []interface{}) {
	if len(backlog) == 0 {
		return
	}

	saved, err := self.logCollection.InsertMany(context.Background(), backlog)
	if err != nil {
		panic(err)
	}
    fmt.Printf("Saved %d log lines\n", len(saved.InsertedIDs))
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
