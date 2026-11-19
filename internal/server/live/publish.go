package live

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"

	"github.com/markojerkic/svarog/internal/server/logstore"
)

type sourceScope struct {
	scope    logstore.Scope
	instance string
}

func (l *Logs) publishLoop() {
	for {
		select {
		case <-l.done:
			return
		case scope := <-l.invalidations:
			l.invalidate(scope)
		case job := <-l.publish:
			groups := make(map[sourceScope][]logstore.Row)
			for _, row := range job.rows {
				source := sourceScope{scope: row.Scope, instance: row.Instance}
				groups[source] = append(groups[source], row)
			}
			for source, rows := range groups {
				if err := l.publishSource(source, rows); err != nil {
					warnPublish(err)
					l.invalidate(source.scope)
				}
			}
			l.mu.Lock()
			l.publishRows -= len(job.rows)
			l.publishBytes -= job.bytes
			l.mu.Unlock()
		}
	}
}

func (l *Logs) publishSource(source sourceScope, rows []logstore.Row) error {
	// Base64 expands the name. Bound it before building a subject, so an
	// untrusted oversized hostname cannot exceed the broker's control line.
	if len(source.instance) > maxInstanceBytes {
		return fmt.Errorf("committed live instance exceeds subject size limit")
	}
	batch := committedBatch{
		Version: 1, ID: uuid.NewString(),
		ProjectID: source.scope.ProjectID.String(), ClientID: source.scope.ClientID.String(),
		Instance: source.instance, Rows: make([]committedRow, 0, maxBatchRows),
	}
	header, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	overhead := len(header)
	size := overhead
	limit := min(maxBatchBytes, int(l.conn.MaxPayload()))
	flush := func() error {
		if len(batch.Rows) == 0 {
			return nil
		}
		payload, err := json.Marshal(batch)
		if err != nil {
			return err
		}
		if len(payload) > limit {
			return fmt.Errorf("committed live batch exceeds payload limit")
		}
		if err := l.conn.Publish(subjectPrefix(source.scope)+instanceToken(source.instance), payload); err != nil {
			return err
		}
		batch.ID = uuid.NewString()
		batch.Rows = batch.Rows[:0]
		size = overhead
		return nil
	}
	for _, row := range rows {
		committed := committedRow{ID: row.ID, OccurredUs: row.OccurredAt.UnixMicro(), Sequence: row.Sequence, Message: row.Message}
		encoded, err := json.Marshal(committed)
		if err != nil {
			return err
		}
		if len(encoded)+overhead > limit {
			return fmt.Errorf("committed live row exceeds payload limit")
		}
		// One byte per row safely accounts for array commas, including after a flush.
		if len(batch.Rows) == maxBatchRows || size+len(encoded)+1 > limit {
			if err := flush(); err != nil {
				return err
			}
		}
		batch.Rows = append(batch.Rows, committed)
		size += len(encoded) + 1
	}
	return flush()
}

func (l *Logs) invalidateRows(rows []logstore.Row) {
	scopes := make(map[logstore.Scope]struct{})
	for _, row := range rows {
		scopes[row.Scope] = struct{}{}
	}
	l.invalidateScopes(scopes)
}

func (l *Logs) invalidateScopes(scopes map[logstore.Scope]struct{}) {
	for scope := range scopes {
		// Even the overflow path must not perform broker I/O on the database
		// caller. A bounded control queue keeps invalidation best effort too.
		select {
		case <-l.done:
			return
		case l.invalidations <- scope:
		default:
			log.Warn("Live invalidation queue is full", "project_id", scope.ProjectID, "client_id", scope.ClientID)
		}
	}
}

func (l *Logs) invalidate(scope logstore.Scope) {
	batch := committedBatch{Version: 1, ID: uuid.NewString(), ProjectID: scope.ProjectID.String(), ClientID: scope.ClientID.String(), Resync: true}
	payload, err := json.Marshal(batch)
	if err == nil {
		err = l.conn.Publish(subjectPrefix(scope)+"r", payload)
	}
	if err != nil {
		log.Warn("Could not invalidate committed live log views", "error", err)
	}
}
