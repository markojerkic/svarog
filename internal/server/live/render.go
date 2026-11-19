package live

import (
	"bytes"
	"context"
	"time"

	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/markojerkic/svarog/internal/server/ui/components/logs"
)

func (l *Logs) renderLoop() {
	for {
		select {
		case <-l.done:
			return
		case job := <-l.updates:
			l.render(job.group, job.batch)
			l.mu.Lock()
			l.renderRows -= len(job.batch.Rows)
			l.renderBytes -= job.bytes
			l.mu.Unlock()
		}
	}
}

func (l *Logs) render(group *watchGroup, batch committedBatch) {
	viewers := group.viewersFor(batch.Instance)
	if len(viewers) == 0 {
		return
	}
	fail := func() {
		for _, viewer := range viewers {
			viewer.fail(ErrLiveResync)
		}
	}
	var frame bytes.Buffer
	if err := logs.OobSwapLogLines(nil).Render(context.Background(), &frame); err != nil {
		fail()
		return
	}
	overhead := frame.Len()
	size := overhead
	fragments := make([]string, 0, maxBatchRows)
	flush := func() bool {
		if len(fragments) == 0 {
			return true
		}
		frame.Reset()
		if err := logs.OobSwapLogLines(fragments).Render(context.Background(), &frame); err != nil || frame.Len() > maxBatchBytes {
			fail()
			return false
		}
		// The next flush reuses frame's buffer. Viewers must retain an immutable
		// copy; the same copy is shared by all local selectors matching this source.
		payload := bytes.Clone(frame.Bytes())
		for _, viewer := range viewers {
			viewer.enqueue(payload)
		}
		fragments = fragments[:0]
		size = overhead
		return true
	}
	for _, row := range batch.Rows {
		stored := logstore.Row{
			ID: row.ID, OccurredAt: time.UnixMicro(row.OccurredUs), Sequence: row.Sequence, Message: row.Message,
			Scope: group.scope, Instance: batch.Instance,
		}
		var rendered bytes.Buffer
		if err := logs.LogLine(logs.LogLineProps{Row: stored}).Render(context.Background(), &rendered); err != nil || rendered.Len()+overhead > maxBatchBytes {
			fail()
			return
		}
		if len(fragments) == maxBatchRows || size+rendered.Len() > maxBatchBytes {
			if !flush() {
				return
			}
		}
		fragments = append(fragments, rendered.String())
		size += rendered.Len()
	}
	flush()
}
