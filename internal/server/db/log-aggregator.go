package db

import (
	"context"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/internal/lib/backlog"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/types"
)

type LogLineWithHost struct {
	*rpc.LogLine
	ClientId string
	Hostname string
}

type AggregatingLogServer interface {
	Run(ctx context.Context, logIngestChannel <-chan LogLineWithHost)
	IsBacklogEmpty() bool
	BacklogCount() int
}

type LogServer struct {
	ctx        context.Context
	logService LogService

	logs    chan types.StoredLog
	backlog backlog.Backlog[types.StoredLog]
}
type AvailableClient struct {
	Client   types.StoredClient
	IsOnline bool
}

var _ AggregatingLogServer = &LogServer{}

func NewLogServer(dbClient LogService) AggregatingLogServer {
	return &LogServer{
		logService: dbClient,
		logs:       make(chan types.StoredLog, 1024*1024),
		backlog:    backlog.NewBacklog[types.StoredLog](1024 * 1024),
	}
}

func (self *LogServer) dumpBacklog(ctx context.Context, logsToSave []types.StoredLog) {
	err := self.logService.SaveLogs(ctx, logsToSave)
	if err != nil {
		slog.Error("Could not save logs", "error", err)
		panic(err)
	}
}

func (self *LogServer) Run(ctx context.Context, logIngestChannel <-chan LogLineWithHost) {
	slog.Debug("Starting log server")
	interval := time.NewTicker(5 * time.Second)
	defer interval.Stop()

outer:
	for {
		select {
		case line := <-logIngestChannel:
			logLine := types.StoredLog{
				LogLine:        line.Message,
				Timestamp:      line.Timestamp,
				SequenceNumber: line.Sequence,
				Client: types.StoredClient{
					ClientId:  line.ClientId,
					IpAddress: line.Hostname,
				},
			}
			self.backlog.AddToBacklog(logLine)

		case logsToSave := <-self.backlog.GetLogs():
			go self.dumpBacklog(self.ctx, logsToSave)

		case <-interval.C:
			self.backlog.ForceDump()

		case <-ctx.Done():
			slog.Debug("Context done")
			break outer
		}
	}
}

func (self *LogServer) IsBacklogEmpty() bool {
	return self.backlog.IsEmpty()
}

func (self *LogServer) BacklogCount() int {
	return self.backlog.Count()
}
