package websocket

import (
	"context"
	"errors"
	"sync"

	"github.com/nats-io/nats.go"
)

const (
	subscriptionMaxMessages = 100
	subscriptionMaxBytes    = 1 << 20
)

var ErrLiveResync = errors.New("live log view requires a fresh snapshot")

// LogSubscription owns its queue. Callbacks never block on a viewer, and only
// the done signal is closed, so an in-flight callback cannot send to a closed channel.
type LogSubscription struct {
	mu       sync.Mutex
	messages chan []byte
	done     chan struct{}
	bytes    int
	err      error
	sub      *nats.Subscription
}

func newLogSubscription() *LogSubscription {
	return &LogSubscription{
		messages: make(chan []byte, subscriptionMaxMessages),
		done:     make(chan struct{}),
	}
}

func (s *LogSubscription) attach(sub *nats.Subscription) {
	s.mu.Lock()
	s.sub = sub
	closed := s.err != nil
	s.mu.Unlock()
	if closed {
		sub.Unsubscribe()
	}
}

func (s *LogSubscription) enqueue(message []byte) {
	s.mu.Lock()
	if s.err != nil {
		s.mu.Unlock()
		return
	}
	if s.bytes+len(message) <= subscriptionMaxBytes {
		select {
		case s.messages <- message:
			s.bytes += len(message)
			s.mu.Unlock()
			return
		default:
		}
	}
	s.mu.Unlock()
	s.fail(ErrLiveResync)
}

func (s *LogSubscription) Next(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, s.failure()
	case message := <-s.messages:
		s.mu.Lock()
		s.bytes -= len(message)
		err := s.err
		s.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return message, nil
	}
}

func (s *LogSubscription) failure() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *LogSubscription) fail(err error) {
	s.mu.Lock()
	if s.err != nil {
		s.mu.Unlock()
		return
	}
	s.err = err
	close(s.done)
	sub := s.sub
	s.mu.Unlock()
	if sub != nil {
		sub.Unsubscribe()
	}
}

func (s *LogSubscription) Close() {
	s.fail(context.Canceled)
}
