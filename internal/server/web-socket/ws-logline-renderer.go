package websocket

import (
	"bytes"
	"context"

	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/components/logs"
)

type WsLogLineRenderer struct {
	updates chan types.StoredLog
}

const workerCount = 10

func NewWsLogLineRenderer() *WsLogLineRenderer {
	renderer := &WsLogLineRenderer{}

	for range workerCount {
		go renderer.run(context.Background())
	}

	return renderer
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

}
