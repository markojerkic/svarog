package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/markojerkic/svarog/internal/lib/natsconn"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Service owns every fetched message until commit/ACK, rejection or shutdown.
// DB and NATS connections are borrowed; their owner closes them after Shutdown.
type Service struct {
	connection *natsconn.NatsConnection
	consumer   jetstream.Consumer
	queries    *sqlc.Queries
	writer     LogWriter
	ackWait    time.Duration
	stop       chan struct{}
	done       chan struct{}
	mu         sync.Mutex
	started    bool
	stopping   bool
	cancel     context.CancelFunc
	runErr     error
	pendingMu  sync.Mutex
	pending    map[*pendingLog]struct{}
	rejected   map[rejectReason]uint64
}

// Open validates options and prepares durable broker state synchronously.
func Open(ctx context.Context, opts Options) (*Service, error) {
	if opts.Connection == nil || opts.Connection.Conn == nil || opts.Connection.JetStream == nil || opts.Pool == nil || opts.Writer == nil {
		return nil, errors.New("ingest requires NATS, JetStream, PostgreSQL and a log writer")
	}
	if opts.AckWait == 0 {
		opts.AckWait = time.Minute
	}
	if opts.AckWait < 30*time.Millisecond || opts.StreamMaxBytes < 0 {
		return nil, errors.New("ingest AckWait must be at least 30ms and StreamMaxBytes must not be negative")
	}
	consumer, err := prepareBroker(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Service{
		connection: opts.Connection, consumer: consumer, queries: sqlc.New(opts.Pool),
		writer: opts.Writer, ackWait: opts.AckWait,
		stop: make(chan struct{}), done: make(chan struct{}),
		pending: make(map[*pendingLog]struct{}), rejected: make(map[rejectReason]uint64),
	}, nil
}

// Run is called once. Canceling ctx is a hard stop; use Shutdown to drain first.
func (s *Service) Run(ctx context.Context) (err error) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("ingest Run may only be called once")
	}
	s.started = true
	ctx, s.cancel = context.WithCancel(ctx)
	cancel := s.cancel
	s.mu.Unlock()
	heartbeatDone := make(chan struct{})
	go func() { defer close(heartbeatDone); s.heartbeat(ctx) }()
	defer func() {
		cancel()
		<-heartbeatDone
		s.releasePending()
		s.mu.Lock()
		s.runErr = err
		close(s.done)
		s.mu.Unlock()
	}()
	return s.run(ctx)
}

func (s *Service) run(ctx context.Context) error {
	var batch []*pendingLog
	var bytes int
	var started time.Time
	var retry fetchRetry
	flush := func() error {
		err := s.commitBatch(ctx, batch)
		batch = nil
		bytes = 0
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.isStopping() {
			return flush()
		}
		wait := fetchWait
		if len(batch) > 0 {
			remaining := time.Until(started.Add(batchWait))
			if remaining < minFetchWait {
				if err := flush(); err != nil {
					return err
				}
				continue
			}
			wait = min(wait, remaining)
		}
		if s.connection.Conn.IsClosed() {
			return fmt.Errorf("fetch ingest messages: %w", nats.ErrConnectionClosed)
		}
		// A pull queued in the reconnect buffer would be replayed for an inbox
		// that no longer waits, so wait for the connection instead of fetching.
		if delay := time.Until(retry.at); delay > 0 || !s.connection.Conn.IsConnected() {
			if delay > 0 {
				wait = min(wait, delay)
			}
			s.pause(ctx, wait)
			continue
		}
		received, fetchErr := s.fetch(min(fetchRows, maxBatchRows-len(batch)), wait)
		// Messages delivered before a pull failure are already owned here.
		for _, p := range received {
			input, reason, err := s.decode(ctx, p.message)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				slog.Warn("Log client lookup failed; retrying delivery", "error", err)
				s.finish(p, "retry", func() error { return p.message.NakWithDelay(time.Second) })
				continue
			}
			if reason != "" {
				s.reject(p, reason)
				continue
			}
			p.input = input
			if len(batch) > 0 && bytes+p.wireBytes > maxBatchBytes {
				if err := flush(); err != nil {
					return err
				}
			}
			if len(batch) == 0 {
				started = time.Now()
			}
			batch = append(batch, p)
			bytes += p.wireBytes
			if len(batch) >= maxBatchRows || bytes >= maxBatchBytes || time.Since(started) >= batchWait {
				if err := flush(); err != nil {
					return err
				}
			}
		}
		if fetchErr == nil {
			retry = fetchRetry{}
			continue
		}
		if err := s.fetchFailed(ctx, &retry, fetchErr); err != nil {
			return err
		}
	}
}

// fetch registers the whole pull before any slow validation or batch commit,
// so every message already in this process receives progress heartbeats.
// FetchMaxWait lets the server end the pull; a client-side deadline could
// expire first and discard a message that was just delivered.
func (s *Service) fetch(rows int, wait time.Duration) ([]*pendingLog, error) {
	messages, err := s.consumer.Fetch(rows, jetstream.FetchMaxWait(wait))
	if err != nil {
		return nil, err
	}
	var received []*pendingLog
	for msg := range messages.Messages() {
		p := &pendingLog{message: msg, wireBytes: len(msg.Data()) + len(msg.Subject()) + len(msg.Reply()) + 256}
		for key, values := range msg.Headers() {
			for _, value := range values {
				p.wireBytes += len(key) + len(value) + 4
			}
		}
		s.pendingMu.Lock()
		s.pending[p] = struct{}{}
		s.pendingMu.Unlock()
		received = append(received, p)
	}
	return received, messages.Error()
}

func (s *Service) pause(ctx context.Context, wait time.Duration) {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-s.stop:
	case <-timer.C:
	}
}

func (s *Service) isStopping() bool {
	select {
	case <-s.stop:
		return true
	default:
		return false
	}
}

// Shutdown stops new pulls, commits accepted work and waits before borrowed
// connections may close. Deadline expiration cancels DB calls, then joins Run.
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if !s.stopping {
		s.stopping = true
		close(s.stop)
	}
	started, cancel := s.started, s.cancel
	s.mu.Unlock()
	if !started {
		return nil
	}
	select {
	case <-s.done:
	case <-ctx.Done():
		cancel()
		<-s.done
		return ctx.Err()
	}
	s.mu.Lock()
	err := s.runErr
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if err := s.connection.Conn.FlushWithContext(ctx); err != nil {
		return fmt.Errorf("flush ingest acknowledgements: %w", err)
	}
	return nil
}

func (s *Service) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(min(s.ackWait/3, 10*time.Second))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pendingMu.Lock()
			for p := range s.pending {
				if err := p.message.InProgress(); err != nil {
					slog.Warn("Could not extend log delivery deadline", "error", err)
				}
			}
			s.pendingMu.Unlock()
		}
	}
}

func (s *Service) finish(p *pendingLog, action string, complete func() error) {
	// Serialize terminal actions with heartbeat so InProgress cannot be sent
	// after ACK/Term for this delivery.
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if err := complete(); err != nil {
		slog.Warn("Could not complete log delivery; broker may redeliver", "action", action, "error", err)
	}
	delete(s.pending, p)
}

func (s *Service) reject(p *pendingLog, reason rejectReason) {
	s.rejected[reason]++
	slog.Warn("Rejected log delivery", "reason", reason, "rejected_total", s.rejected[reason], "subject", p.message.Subject())
	s.finish(p, "terminate", func() error { return p.message.TermWithReason(string(reason)) })
}

func (s *Service) releasePending() {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	for p := range s.pending {
		if err := p.message.Nak(); err != nil {
			slog.Warn("Could not release uncommitted log delivery", "error", err)
		}
		delete(s.pending, p)
	}
}
