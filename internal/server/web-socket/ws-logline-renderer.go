package websocket

import (
	"bytes"
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/components/logs"
)

const (
	workerCount      = 10
	renderQueueRows  = 8192
	renderQueueBytes = 8 << 20
	logBatchRows     = 100
	logBatchBytes    = 128 << 10
)

type renderBatch struct {
	rows  []types.StoredLog
	bytes int
}

type logScope struct {
	projectID string
	clientID  string
}

type WsLogLineRenderer struct {
	updates  chan renderBatch
	watchHub *WatchHub
	done     chan struct{}
	workers  sync.WaitGroup
	mu       sync.Mutex
	closed   bool
	rows     int
	bytes    int
}

func NewWsLogLineRenderer(watchHub *WatchHub) *WsLogLineRenderer {
	renderer := &WsLogLineRenderer{
		updates:  make(chan renderBatch, 64),
		watchHub: watchHub,
		done:     make(chan struct{}),
	}
	for range workerCount {
		renderer.workers.Add(1)
		go renderer.run()
	}
	return renderer
}

// RenderBatch takes a bounded copy after the DB insert. A full live-view queue
// invalidates the view instead of delaying persistence or retrying saved rows.
func (w *WsLogLineRenderer) RenderBatch(ctx context.Context, rows []types.StoredLog) {
	if len(rows) == 0 {
		return
	}
	size := 0
	for _, row := range rows {
		size += len(row.LogLine) + len(row.Client.InstanceId) + 256
	}
	w.mu.Lock()
	if !w.closed && ctx.Err() == nil && w.rows+len(rows) <= renderQueueRows && w.bytes+size <= renderQueueBytes {
		batch := renderBatch{rows: slices.Clone(rows), bytes: size}
		select {
		case w.updates <- batch:
			w.rows += len(rows)
			w.bytes += size
			w.mu.Unlock()
			return
		default:
		}
	}
	w.mu.Unlock()
	scopes := make(map[logScope]struct{})
	for _, row := range rows {
		scopes[logScope{row.Client.ProjectId, row.Client.ClientId}] = struct{}{}
	}
	for scope := range scopes {
		w.resync(scope, "render queue unavailable")
	}
}

func (w *WsLogLineRenderer) run() {
	defer w.workers.Done()
	for {
		select {
		case <-w.done:
			return
		case batch := <-w.updates:
			groups := make(map[logScope][]types.StoredLog)
			for _, row := range batch.rows {
				scope := logScope{row.Client.ProjectId, row.Client.ClientId}
				groups[scope] = append(groups[scope], row)
			}
			for scope, rows := range groups {
				w.render(scope, rows)
			}
			w.mu.Lock()
			w.rows -= len(batch.rows)
			w.bytes -= batch.bytes
			w.mu.Unlock()
		}
	}
}

func (w *WsLogLineRenderer) render(scope logScope, rows []types.StoredLog) {
	limit := min(logBatchBytes, int(w.watchHub.conn.MaxPayload()))
	var frame bytes.Buffer
	if err := logs.OobSwapLogLines(nil).Render(context.Background(), &frame); err != nil {
		w.resync(scope, err.Error())
		return
	}
	overhead := frame.Len()
	fragments := make([]string, 0, logBatchRows)
	size := overhead
	flush := func() bool {
		if len(fragments) == 0 {
			return true
		}
		frame.Reset()
		if err := logs.OobSwapLogLines(fragments).Render(context.Background(), &frame); err != nil {
			w.resync(scope, err.Error())
			return false
		}
		if frame.Len() > limit {
			w.resync(scope, "rendered batch exceeds live payload limit")
			return false
		}
		if err := w.watchHub.SendLogLine(scope.projectID, scope.clientID, frame.Bytes()); err != nil {
			w.resync(scope, err.Error())
			return false
		}
		fragments = fragments[:0]
		size = overhead
		return true
	}
	for _, row := range rows {
		// PostgreSQL stores microseconds; live and historical sorting must agree.
		row.Timestamp = row.Timestamp.Truncate(time.Microsecond)
		var rendered bytes.Buffer
		if err := logs.LogLine(logs.LogLineProps{LogLine: row}).Render(context.Background(), &rendered); err != nil {
			w.resync(scope, err.Error())
			return
		}
		if rendered.Len()+overhead > limit {
			w.resync(scope, "rendered log line exceeds live batch limit")
			return
		}
		if len(fragments) == logBatchRows || size+rendered.Len() > limit {
			if !flush() {
				return
			}
		}
		fragments = append(fragments, rendered.String())
		size += rendered.Len()
	}
	flush()
}

func (w *WsLogLineRenderer) resync(scope logScope, reason string) {
	slog.Warn("Live log view requires refresh", "project_id", scope.projectID, "client_id", scope.clientID, "reason", reason)
	if err := w.watchHub.Resync(scope.projectID, scope.clientID); err != nil {
		slog.Error("Could not invalidate live log view", "error", err)
	}
}

func (w *WsLogLineRenderer) Close() {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.done)
	}
	w.mu.Unlock()
	w.workers.Wait()
}
