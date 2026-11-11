package live

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/nats-io/nats.go"
)

type publishJob struct {
	rows  []types.StoredLog
	bytes int
}

type renderJob struct {
	group *watchGroup
	batch committedBatch
	bytes int
}

// Logs owns live subscriptions, lazy rendering and publication budgets. It
// borrows a dedicated NATS connection; the composition root closes that last.
type Logs struct {
	conn          *nats.Conn
	mu            sync.Mutex
	closeOnce     sync.Once
	groups        map[Scope]*watchGroup
	closed        bool
	publish       chan publishJob
	invalidations chan Scope
	updates       chan renderJob
	publishRows   int
	publishBytes  int
	renderRows    int
	renderBytes   int
	done          chan struct{}
	workers       sync.WaitGroup
}

func New(conn *nats.Conn) *Logs {
	l := &Logs{
		conn:          conn,
		groups:        make(map[Scope]*watchGroup),
		publish:       make(chan publishJob, maxQueueJobs),
		invalidations: make(chan Scope, maxQueueJobs),
		updates:       make(chan renderJob, maxQueueJobs),
		done:          make(chan struct{}),
	}
	l.workers.Go(l.publishLoop)
	for range 10 {
		l.workers.Go(l.renderLoop)
	}
	status := conn.StatusChanged(nats.RECONNECTING, nats.CLOSED)
	l.workers.Go(func() {
		defer conn.RemoveStatusListener(status)
		for {
			select {
			case <-l.done:
				return
			case <-status:
				l.mu.Lock()
				groups := make([]*watchGroup, 0, len(l.groups))
				for _, group := range l.groups {
					groups = append(groups, group)
				}
				l.mu.Unlock()
				for _, group := range groups {
					group.fail(ErrLiveResync)
				}
			}
		}
	})
	return l
}

// AfterCommit accepts a bounded copy, regardless of this replica's viewers.
// Failure is a live-view failure, never a reason to retry a committed insert.
func (l *Logs) AfterCommit(ctx context.Context, rows []types.StoredLog) {
	if len(rows) == 0 {
		return
	}
	size := 0
	for _, row := range rows {
		size += len(row.LogLine) + len(row.Client.InstanceId) + 256
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	if ctx.Err() == nil && l.publishRows+len(rows) <= maxQueueRows && l.publishBytes+size <= maxQueueBytes {
		select {
		case l.publish <- publishJob{rows: slices.Clone(rows), bytes: size}:
			l.publishRows += len(rows)
			l.publishBytes += size
			l.mu.Unlock()
			return
		default:
		}
	}
	l.mu.Unlock()
	l.invalidateRows(rows)
}

// AfterUncertainCommit is for a write whose outcome is unknown. The rows may be
// committed without publication, and a retry will not publish existing rows,
// so matching views must stop visibly instead of silently missing them.
func (l *Logs) AfterUncertainCommit(rows []types.StoredLog) {
	l.invalidateRows(rows)
}

// Open waits for broker readiness. ctx bounds opening, not the lifetime of
// the returned Session; its owner must Close it. Scope must already be authorized.
func (l *Logs) Open(ctx context.Context, scope Scope, selector Selector) (*Session, error) {
	if scope.ProjectID == uuid.Nil || scope.ClientID == uuid.Nil {
		return nil, errors.New("invalid live log scope")
	}
	selector, err := NewSelector(selector.Instances)
	if err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !l.conn.IsConnected() {
			return nil, ErrClosed
		}
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			return nil, ErrClosed
		}
		g := l.groups[scope]
		if g == nil {
			g = &watchGroup{owner: l, scope: scope, viewers: make(map[*Session]Selector), subscriptions: make(map[string]*sourceSubscription)}
			l.groups[scope] = g
		}
		l.mu.Unlock()
		g.change.Lock()
		select {
		case <-l.done:
			g.change.Unlock()
			return nil, ErrClosed
		default:
		}
		g.mu.Lock()
		closed := g.closed
		empty := len(g.viewers) == 0
		if ctx.Err() != nil && empty {
			g.closed = true
			closed = true
		}
		g.mu.Unlock()
		if closed {
			l.removeGroup(g)
			g.change.Unlock()
			continue
		}
		if err := ctx.Err(); err != nil {
			g.change.Unlock()
			return nil, err
		}
		var session *Session
		session = newSession(func() { l.release(g, session) })
		g.mu.Lock()
		if g.closed {
			g.mu.Unlock()
			g.change.Unlock()
			continue
		}
		g.viewers[session] = selector
		g.mu.Unlock()
		err := g.reconcile(ctx)
		if err != nil {
			g.fail(ErrLiveResync)
		}
		g.change.Unlock()
		if err != nil {
			return nil, err
		}
		return session, nil
	}
}

func (l *Logs) release(g *watchGroup, session *Session) {
	g.change.Lock()
	defer g.change.Unlock()
	g.mu.Lock()
	delete(g.viewers, session)
	empty := len(g.viewers) == 0
	closed := g.closed
	if empty {
		g.closed = true
	}
	g.mu.Unlock()
	if empty || closed {
		l.removeGroup(g)
		return
	}
	if err := g.reconcile(context.Background()); err != nil {
		g.fail(ErrLiveResync)
	}
}

// g.change must be held. A retiring group can coexist briefly with its
// replacement. Never hold the publication budget lock while calling NATS.
func (l *Logs) removeGroup(g *watchGroup) {
	l.mu.Lock()
	if l.groups[g.scope] == g {
		delete(l.groups, g.scope)
	}
	l.mu.Unlock()
	for subject, source := range g.subscriptions {
		source.retired.Store(true)
		source.sub.Unsubscribe()
		delete(g.subscriptions, subject)
	}
}

func (l *Logs) enqueue(job renderJob) {
	l.mu.Lock()
	if !l.closed && l.renderRows+len(job.batch.Rows) <= maxQueueRows && l.renderBytes+job.bytes <= maxQueueBytes {
		select {
		case l.updates <- job:
			l.renderRows += len(job.batch.Rows)
			l.renderBytes += job.bytes
			l.mu.Unlock()
			return
		default:
		}
	}
	l.mu.Unlock()
	job.group.fail(ErrLiveResync)
}

func (l *Logs) Close() {
	l.closeOnce.Do(func() {
		l.mu.Lock()
		l.closed = true
		close(l.done)
		groups := make([]*watchGroup, 0, len(l.groups))
		for _, group := range l.groups {
			groups = append(groups, group)
		}
		l.mu.Unlock()
		for _, group := range groups {
			group.change.Lock()
			group.fail(ErrClosed)
			l.removeGroup(group)
			group.change.Unlock()
		}
		l.workers.Wait()
	})
}

func warnPublish(err error) {
	slog.Warn("Could not publish committed live logs; a fresh view may be required", "error", err)
}
