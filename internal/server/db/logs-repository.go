package db

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/backlog"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/types"
)

type LogRepository interface {
	SaveLogs(ctx context.Context, logs []types.StoredLog) error
	GetLogs(ctx context.Context, clientId string, instances *[]string, pageSize int64, cursor *LastCursor) ([]types.StoredLog, error)
	GetClients(ctx context.Context) ([]types.Client, error)
	GetInstances(ctx context.Context, clientId string) ([]string, error)
	SearchLogs(ctx context.Context, query string, clientId string, instances *[]string, pageSize int64, lastCursor *LastCursor) ([]types.StoredLog, error)
}

type LastCursor struct {
	Timestamp      time.Time
	SequenceNumber int
	IsBackward     bool
}

type LogLineWithIp struct {
	*rpc.LogLine
	Ip string
}

type AggregatingLogServer interface {
	Run(ctx context.Context, logIngestChannel <-chan LogLineWithIp)
	IsBacklogEmpty() bool
	BacklogCount() int
}

type LogServer struct {
	ctx        context.Context
	repository LogRepository

	logs    chan types.StoredLog
	backlog backlog.Backlog[types.StoredLog]
}
type AvailableClient struct {
	Client   types.StoredClient
	IsOnline bool
}

var _ AggregatingLogServer = &LogServer{}

func NewLogServer(dbClient LogRepository) AggregatingLogServer {
	return &LogServer{
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

func (self *LogServer) Run(ctx context.Context, logIngestChannel <-chan LogLineWithIp) {
	log.Debug("Starting log server")
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
					IpAddress: line.Ip,
				},
			}
			self.backlog.AddToBacklog(logLine)

		case logsToSave := <-self.backlog.GetLogs():
			go self.dumpBacklog(self.ctx, logsToSave)

		case <-interval.C:
			self.backlog.ForceDump()

		case <-ctx.Done():
			log.Debug("Context done")
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
