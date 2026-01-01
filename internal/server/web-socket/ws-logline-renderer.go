package websocket

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/components/logs"
)

type WsLogLineRenderer struct {
	updates  chan types.StoredLog
	watchHub *WatchHub
}

const workerCount = 10

func NewWsLogLineRenderer(watchHub *WatchHub) *WsLogLineRenderer {
	renderer := &WsLogLineRenderer{
		updates: make(chan types.StoredLog, 1024),
	}

	for range workerCount {
		go renderer.run(context.Background())
	}

	return renderer
}

func (w *WsLogLineRenderer) Render(ctx context.Context, logLine types.StoredLog) {
	w.updates <- logLine
}

func (w *WsLogLineRenderer) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case logLine := <-w.updates:
			w.render(logLine)
		}
	}
}

func (w *WsLogLineRenderer) render(logLine types.StoredLog) {
	var buf bytes.Buffer
	logs.OobSwapLogLine(logs.LogLineProps{LogLine: logLine}).Render(context.Background(), &buf)
	w.watchHub.SendLogLine(logLine.Client.ProjectId, logLine.Client.ClientId, buf.Bytes())
}
