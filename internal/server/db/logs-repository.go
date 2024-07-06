package db

import (
	"context"
	"log"
	"log/slog"
	"time"

	rpc "github.com/markojerkic/svarog/internal/proto"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LogRepository interface {
	SaveLogs(logs []interface{}) error
	GetLogs(clientId string, pageSize int64, cursor *LastCursor) ([]StoredLog, error)
	GetClients() ([]AvailableClient, error)
}

type LastCursor struct {
	Timestamp      time.Time
	SequenceNumber int
	IsBackward     bool
}

type AggregatingLogServer interface {
	Run(logIngestChannel <-chan *rpc.LogLine)
	IsBacklogEmpty() bool
	BacklogCount() int
}

type LogServer struct {
	ctx        context.Context
	repository LogRepository

	logs    chan *StoredLog
	backlog Backlog[any]
}

var _ AggregatingLogServer = &LogServer{}

type AvailableClient struct {
	Client   StoredClient
	IsOnline bool
}

type StoredClient struct {
	ClientId  string `bson:"client_id" json:"clientId"`
	IpAddress string `bson:"ip_address" json:"ipAddress"`
}

type StoredLog struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	LogLine        string             `bson:"log_line"`
	Timestamp      time.Time          `bson:"timestamp"`
	Client         StoredClient       `bson:"client"`
	SequenceNumber int64              `bson:"sequence_number"`
}

func NewLogServer(ctx context.Context, dbClient LogRepository) AggregatingLogServer {
	return &LogServer{
		ctx:        ctx,
		repository: dbClient,
		logs:       make(chan *StoredLog, 1024*1024),
		backlog:    NewBacklog[any](1024 * 1024),
	}
}

func (self *LogServer) dumpBacklog(logsToSave []interface{}) {
	err := self.repository.SaveLogs(logsToSave)
	if err != nil {
		log.Fatalf("Could not save logs: %v", err)
	}
}

func (self *LogServer) Run(logIngestChannel <-chan *rpc.LogLine) {
	slog.Debug("Starting log server")
	for {
		select {
		case line := <-logIngestChannel:
			logLine := &StoredLog{
				LogLine:        line.Message,
				Timestamp:      line.Timestamp.AsTime(),
				SequenceNumber: line.Sequence,
				Client: StoredClient{
					ClientId:  line.Client,
					IpAddress: "::1",
				},
			}
			self.backlog.AddToBacklog(logLine)

		case logsToSave := <-self.backlog.GetLogs():
			go self.dumpBacklog(logsToSave)

		case <-time.After(5 * time.Second):
			slog.Debug("Dumping backlog after timeout")
			self.backlog.ForceDump()

		case <-self.ctx.Done():
			slog.Debug("Context done")
			break
		}
	}
}

func (self *LogServer) IsBacklogEmpty() bool {
	return self.backlog.IsEmpty()
}

func (self *LogServer) BacklogCount() int {
	return self.backlog.Count()
}
