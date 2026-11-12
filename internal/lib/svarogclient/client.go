package svarogclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/lib/svarogclient/config"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/nats-io/nkeys"
)

const queueCapacity = 16

var (
	ErrClosed       = errors.New("Svarog client is closed")
	ErrInvalidEntry = errors.New("invalid log entry")
)

type Entry struct {
	// Message must not contain NUL. Each invalid UTF-8 byte becomes U+FFFD.
	Message string
	// A zero timestamp is set to time.Now once the entry obtains admission,
	// which can be later than the Enqueue call while producers wait.
	Timestamp time.Time
}

type Options struct {
	ConnectionString string
	// InstanceID defaults to SVAROG_INSTANCE_ID, then the hostname.
	InstanceID string
	// Diagnostics defaults to stderr. It must not feed back into this client.
	Diagnostics io.Writer
}

type Client struct {
	config     config.ClientConfig
	instanceID string
	logger     *slog.Logger

	queue    chan []byte
	stopping chan struct{}
	done     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc

	admission sync.Mutex
	sequence  int
	stopOnce  sync.Once
	stateMu   sync.Mutex
	err       error
}

// New validates local configuration and starts a background publisher. It does
// not wait for the broker: unavailable connections retry while the queue fills.
func New(options Options) (*Client, error) {
	cfg, err := config.NewClientConfig(options.ConnectionString)
	if err != nil {
		return nil, err
	}
	if _, err := nkeys.ParseDecoratedJWT([]byte(cfg.Creds)); err != nil {
		return nil, errors.New("credentials must contain a user JWT")
	}
	key, err := nkeys.ParseDecoratedNKey([]byte(cfg.Creds))
	if err != nil {
		return nil, errors.New("credentials must contain a user NKEY seed")
	}
	key.Wipe()

	diagnostics := options.Diagnostics
	if diagnostics == nil {
		diagnostics = os.Stderr
	}
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(diagnostics, &slog.HandlerOptions{Level: level}))
	instanceID := options.InstanceID
	if instanceID == "" {
		instanceID = os.Getenv("SVAROG_INSTANCE_ID")
	}
	if instanceID == "" {
		instanceID, err = os.Hostname()
		if err != nil {
			logger.Warn("Failed to get hostname, using 'unknown'", "error", err)
			instanceID = "unknown"
		}
	}
	if err := logevent.ValidateInstanceID(instanceID); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		config:     cfg,
		instanceID: instanceID,
		logger:     logger,
		queue:      make(chan []byte, queueCapacity),
		stopping:   make(chan struct{}),
		done:       make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}
	go c.run()
	return c, nil
}

// Enqueue waits for space in the bounded queue. Success means local acceptance,
// not broker or database persistence. Shutdown unblocks waiting callers.
// Rejections are also reported locally because slog discards handler errors.
func (c *Client) Enqueue(entry Entry) error {
	truncated, err := c.accept(entry)
	if err != nil {
		c.logger.Error("Log entry was not accepted", "error", err)
	} else if truncated {
		c.logger.Warn("Encoded log line exceeded the payload limit and was truncated", "limit", logevent.MaxPayloadBytes)
	}
	return err
}

func (c *Client) accept(entry Entry) (bool, error) {
	if !utf8.ValidString(entry.Message) {
		// Match encoding/json's earlier repair: one U+FFFD per invalid byte,
		// rather than rejecting the whole line.
		entry.Message = string([]rune(entry.Message))
	}
	// Sequence assignment, encoding and queue insertion share one ordering point.
	c.admission.Lock()
	defer c.admission.Unlock()
	select {
	case <-c.stopping:
		return false, errors.Join(ErrClosed, c.result())
	default:
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	line := rpc.LogLine{
		Message:    entry.Message,
		Timestamp:  entry.Timestamp,
		Sequence:   c.sequence,
		InstanceId: c.instanceID,
	}
	if err := logevent.Validate(line); err != nil {
		return false, fmt.Errorf("%w: %w", ErrInvalidEntry, err)
	}
	payload, truncated, err := logevent.EncodePayload(line)
	if err != nil {
		return false, fmt.Errorf("%w: encode log line: %w", ErrInvalidEntry, err)
	}
	select {
	case <-c.stopping:
		return false, errors.Join(ErrClosed, c.result())
	case c.queue <- payload:
		c.sequence = (c.sequence + 1) % math.MaxInt
		return truncated, nil
	}
}

// Shutdown stops admission and waits for broker acknowledgements for accepted
// entries. Expiry cancels network operations and joins the publisher before
// returning an error. It does not wait for PostgreSQL and cannot recover entries
// left in memory after a timeout or process exit. Concurrent calls are safe;
// any caller's expired context aborts the shared drain.
func (c *Client) Shutdown(ctx context.Context) error {
	c.stopAccepting()
	select {
	case <-c.done:
		return c.result()
	default:
	}
	select {
	case <-c.done:
		return c.result()
	case <-ctx.Done():
		c.setError(fmt.Errorf("client shutdown interrupted before all entries were confirmed: %w", ctx.Err()))
		c.cancel()
		<-c.done
		return c.result()
	}
}

func (c *Client) stopAccepting() {
	c.stopOnce.Do(func() {
		close(c.stopping)
		// Closing stopping releases an Enqueue blocked while holding admission.
		c.admission.Lock()
		close(c.queue)
		c.admission.Unlock()
	})
}

func (c *Client) setError(err error) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.err == nil {
		c.err = err
	}
}

func (c *Client) result() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.err
}
