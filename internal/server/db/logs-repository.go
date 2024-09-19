package db

import (
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/markojerkic/svarog/internal/lib/backlog"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/types"
)

type LogRepository interface {
	SaveLogs(ctx context.Context, logs []types.StoredLog) error
	GetLogs(ctx context.Context, clientId string, pageSize int64, cursor *LastCursor) ([]types.StoredLog, error)
	GetClients(ctx context.Context) ([]AvailableClient, error)
	GetInstances(ctx context.Context, clientId string) ([]string, error)
	SearchLogs(ctx context.Context, query string, clientId string, pageSize int64, lastCursor *LastCursor) ([]types.StoredLog, error)
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

	logs    chan types.StoredLog
	backlog backlog.Backlog[types.StoredLog]
}

var _ AggregatingLogServer = &LogServer{}

type AvailableClient struct {
	Client   types.StoredClient
	IsOnline bool
}

func NewLogServer(ctx context.Context, dbClient LogRepository) AggregatingLogServer {
	return &LogServer{
		ctx:        ctx,
		repository: dbClient,
		logs:       make(chan types.StoredLog, 1024*1024),
		backlog:    backlog.NewBacklog[types.StoredLog](1024 * 1024),
	}
}

func (self *LogServer) dumpBacklog(ctx context.Context, logsToSave []types.StoredLog) {
	err := self.repository.SaveLogs(ctx, logsToSave)
	if err != nil {
		log.Fatalf("Could not save logs: %v", err)
	}
}

func (self *LogServer) Run(logIngestChannel <-chan *rpc.LogLine) {
	slog.Debug("Starting log server")
	interval := time.NewTicker(5 * time.Second)
	defer interval.Stop()

	for {
		select {
		case line := <-logIngestChannel:
			logLine := types.StoredLog{
				LogLine:        line.Message,
				Timestamp:      line.Timestamp.AsTime(),
				SequenceNumber: line.Sequence,
				Client: types.StoredClient{
					ClientId:  line.Client,
					IpAddress: "::1",
				},
			}
			self.backlog.AddToBacklog(logLine)

		case logsToSave := <-self.backlog.GetLogs():
			go self.dumpBacklog(self.ctx, logsToSave)

		case <-interval.C:
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
