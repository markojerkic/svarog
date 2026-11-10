package live

import (
	"context"
	"sync"
)

// Session owns a bounded viewer queue. Only done is closed; a concurrent
// renderer can never send to a closed data channel. HTML is immutable/shared.
type Session struct {
	mu       sync.Mutex
	messages chan []byte
	done     chan struct{}
	bytes    int
	err      error
	release  func()
}

func newSession(release func()) *Session {
	return &Session{
		messages: make(chan []byte, maxViewerMessages),
		done:     make(chan struct{}),
		release:  release,
	}
}

func (s *Session) enqueue(message []byte) {
	s.mu.Lock()
	if s.err != nil {
		s.mu.Unlock()
		return
	}
	if s.bytes+len(message) <= maxViewerBytes {
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

func (s *Session) Next(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		s.mu.Lock()
		defer s.mu.Unlock()
		return nil, s.err
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

func (s *Session) fail(err error) {
	s.mu.Lock()
	if s.err != nil {
		s.mu.Unlock()
		return
	}
	s.err = err
	close(s.done)
	s.mu.Unlock()
	// Reconciliation can wait for the broker. Never run it on a NATS callback
	// or render worker, nor while holding the viewer queue lock.
	go s.release()
}

// Close immediately stops Next. Shared broker interest is released
// asynchronously, and disappears after the last local viewer leaves.
func (s *Session) Close() {
	s.fail(context.Canceled)
}
