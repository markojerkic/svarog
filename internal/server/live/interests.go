package live

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type sourceSubscription struct {
	sub     *nats.Subscription
	retired atomic.Bool
}

type watchGroup struct {
	owner      *Logs
	scope      Scope
	change     sync.Mutex // serialize broker changes for this client only
	mu         sync.Mutex
	viewers    map[*Session]Selector
	closed     bool
	transition bool
	seen       map[string]struct{}
	// Subscription membership is protected by change, not mu.
	subscriptions map[string]*sourceSubscription
}

// reconcile runs under change. Subscribe -> broker flush -> drain old
// coverage avoids a routing gap. Publication IDs suppress overlap before any
// rendering. The overlap cache is bounded; exhaustion invalidates the view.
func (g *watchGroup) reconcile(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return ErrLiveResync
	}
	desired := make(map[string]struct{})
	all := false
	for _, selector := range g.viewers {
		if len(selector.Instances) == 0 {
			all = true
			break
		}
		for _, instance := range selector.Instances {
			desired[g.scope.prefix()+instanceToken(instance)] = struct{}{}
		}
	}
	if all {
		desired = map[string]struct{}{g.scope.prefix() + "*": {}}
	} else {
		// Exact-source viewers must also learn about a publisher failure
		// affecting a whole committed client batch.
		desired[g.scope.prefix()+"r"] = struct{}{}
	}
	changed := len(desired) != len(g.subscriptions)
	for subject := range desired {
		if g.subscriptions[subject] == nil {
			changed = true
		}
	}
	if !changed {
		g.mu.Unlock()
		return nil
	}
	g.transition = true
	g.seen = make(map[string]struct{})
	g.mu.Unlock()

	for subject := range desired {
		if g.subscriptions[subject] != nil {
			continue
		}
		sub, err := g.owner.conn.Subscribe(subject, g.receive)
		if err != nil {
			return err
		}
		source := &sourceSubscription{sub: sub}
		g.subscriptions[subject] = source
		if err := sub.SetPendingLimits(maxViewerMessages, maxViewerBytes); err != nil {
			return err
		}
		status := sub.StatusChanged(nats.SubscriptionSlowConsumer, nats.SubscriptionClosed)
		go func() {
			select {
			case <-g.owner.done:
				return
			case event := <-status:
				if event == nats.SubscriptionSlowConsumer || !source.retired.Load() {
					g.fail(ErrLiveResync)
				}
			}
		}()
	}
	if err := g.owner.conn.FlushWithContext(ctx); err != nil {
		return err
	}
	retiring := make(map[string]<-chan nats.SubStatus)
	for subject, source := range g.subscriptions {
		if _, keep := desired[subject]; keep {
			continue
		}
		source.retired.Store(true)
		closed := source.sub.StatusChanged(nats.SubscriptionClosed)
		if err := source.sub.Drain(); err != nil {
			return err
		}
		retiring[subject] = closed
	}
	// Start every drain before waiting: a large exact-source union must not
	// pay the library's drain polling interval once per subscription.
	for subject, closed := range retiring {
		select {
		case <-closed:
		case <-ctx.Done():
			return ctx.Err()
		}
		delete(g.subscriptions, subject)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return ErrLiveResync
	}
	g.transition = false
	g.seen = nil
	return nil
}

func (g *watchGroup) receive(message *nats.Msg) {
	var batch committedBatch
	if len(message.Data) > maxBatchBytes || json.Unmarshal(message.Data, &batch) != nil ||
		batch.Version != 1 || batch.ProjectID != g.scope.ProjectID.String() ||
		batch.ClientID != g.scope.ClientID.String() || len(batch.Rows) > maxBatchRows {
		g.fail(ErrLiveResync)
		return
	}
	if _, err := uuid.Parse(batch.ID); err != nil {
		g.fail(ErrLiveResync)
		return
	}
	if batch.Resync {
		g.fail(ErrLiveResync)
		return
	}
	if message.Subject != g.scope.prefix()+instanceToken(batch.Instance) || len(batch.Rows) == 0 {
		g.fail(ErrLiveResync)
		return
	}
	for _, row := range batch.Rows {
		if row.ID <= 0 {
			g.fail(ErrLiveResync)
			return
		}
	}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return
	}
	if g.transition {
		if _, duplicate := g.seen[batch.ID]; duplicate {
			g.mu.Unlock()
			return
		}
		if len(g.seen) >= maxTransitionBatches {
			g.mu.Unlock()
			g.fail(ErrLiveResync)
			return
		}
		g.seen[batch.ID] = struct{}{}
	}
	interested := false
	for _, selector := range g.viewers {
		if selector.matches(batch.Instance) {
			interested = true
			break
		}
	}
	g.mu.Unlock()
	if interested {
		g.owner.enqueue(renderJob{group: g, batch: batch, bytes: len(message.Data)})
	}
}

func (g *watchGroup) viewersFor(instance string) []*Session {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil
	}
	viewers := make([]*Session, 0, len(g.viewers))
	for viewer, selector := range g.viewers {
		if selector.matches(instance) {
			viewers = append(viewers, viewer)
		}
	}
	return viewers
}

func (g *watchGroup) fail(err error) {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return
	}
	g.closed = true
	viewers := g.viewers
	g.viewers = make(map[*Session]Selector)
	g.mu.Unlock()
	for viewer := range viewers {
		viewer.fail(err)
	}
	go func() {
		g.change.Lock()
		defer g.change.Unlock()
		g.owner.removeGroup(g)
	}()
}
