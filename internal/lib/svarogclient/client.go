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

	"github.com/nats-io/nkeys"

	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/markojerkic/svarog/internal/lib/svarogclient/config"
	"github.com/markojerkic/svarog/internal/rpc"
)

const (
	blockingQueueCapacity   = 16
	maxDropQueueBytes       = 16 << 20
	rejectionReportInterval = time.Second
	// DefaultDropQueueCapacity is shared by the Go logging adapter and server.
	DefaultDropQueueCapacity = 1024
	// MaxDropQueueCapacity also bounds the channel allocation when entries are small.
	MaxDropQueueCapacity = 65_536
)

var (
	ErrClosed       = errors.New("Svarog client is closed")
	ErrInvalidEntry = errors.New("invalid log entry")
	ErrQueueFull    = errors.New("Svarog log queue is full")
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
	// DropQueueCapacity enables best-effort delivery with this many queue slots
	// and a fixed 16 MiB encoded payload budget. Zero uses the blocking 16-slot queue.
	DropQueueCapacity int
}

type Client struct {
	config       config.ClientConfig
	instanceID   string
	logger       *slog.Logger
	dropWhenFull bool

	queue    chan []byte
	stopping chan struct{}
	done     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc

	admission   sync.Mutex
	sequence    int
	queuedBytes int // protected by admission; excludes the publication window
	stopOnce    sync.Once
	stateMu     sync.Mutex
	err         error

	rejectionMu   sync.Mutex
	lastReport    time.Time
	rejected      uint64
	lastRejection error
}

// New validates local configuration and starts a background publisher. It does
// not wait for the broker: unavailable connections retry while the queue fills.
func New(options Options) (*Client, error) {
	capacity := options.DropQueueCapacity
	if capacity < 0 || capacity > MaxDropQueueCapacity {
		return nil, fmt.Errorf("drop queue capacity must be between 0 and %d", MaxDropQueueCapacity)
	}
	if capacity == 0 {
		capacity = blockingQueueCapacity
	}
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
		config:       cfg,
		instanceID:   instanceID,
		logger:       logger,
		dropWhenFull: options.DropQueueCapacity > 0,
		queue:        make(chan []byte, capacity),
		stopping:     make(chan struct{}),
		done:         make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}
	go c.run()
	return c, nil
}

// Enqueue waits for space in the bounded queue unless DropQueueCapacity is set.
// Success means local acceptance, not broker or database persistence.
// Shutdown unblocks waiting callers.
// Rejections are also reported locally because slog discards handler errors.
// Rejections are counted and diagnostics are rate limited in both modes.
func (c *Client) Enqueue(entry Entry) error {
	truncated, err := c.accept(entry)
	if err != nil {
		c.reportRejections(err)
	} else if truncated {
		c.logger.Warn("Encoded log line exceeded the payload limit and was truncated", "limit", logevent.MaxPayloadBytes)
	}
	return err
}

// reportRejections records a rejection and reports it when the interval allows.
// A nil error flushes the pending summary when the publisher stops.
func (c *Client) reportRejections(err error) {
	c.rejectionMu.Lock()
	if err != nil {
		c.rejected++
		c.lastRejection = err
	}
	now := time.Now()
	if c.rejected == 0 || (err != nil && now.Sub(c.lastReport) < rejectionReportInterval) {
		c.rejectionMu.Unlock()
		return
	}
	count, latest := c.rejected, c.lastRejection
	c.rejected, c.lastRejection, c.lastReport = 0, nil, now
	c.rejectionMu.Unlock()
	c.logger.Error("Log entries were not accepted", "dropped", count, "latest_error", latest)
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
	// Admission serializes all producers; only the publisher can remove an item.
	if c.dropWhenFull && len(c.queue) == cap(c.queue) {
		return false, ErrQueueFull
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
	if c.dropWhenFull && c.queuedBytes+len(payload) > maxDropQueueBytes {
		return false, ErrQueueFull
	}
	select {
	case <-c.stopping:
		return false, errors.Join(ErrClosed, c.result())
	case c.queue <- payload:
		if c.dropWhenFull {
			c.queuedBytes += len(payload)
		}
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
