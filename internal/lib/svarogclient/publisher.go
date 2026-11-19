package svarogclient

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	connectTimeout         = 2 * time.Second
	publishTimeout         = 5 * time.Second
	maxPendingPublications = 32
	maxPendingBytes        = 4 << 20
	// JetStream's stream max_msg_size rejection cannot succeed on retry.
	jsErrMessageTooLarge jetstream.ErrorCode = 10054
)

type publication struct {
	payload []byte
	id      string
	future  jetstream.PubAckFuture
	err     error // immediate send failure, or an already observed future error
}

func (p *publication) send(js jetstream.JetStream, topic string) {
	p.future, p.err = js.PublishAsync(topic, p.payload, jetstream.WithMsgID(p.id), jetstream.WithRetryAttempts(0))
}

type publisher struct {
	nc *nats.Conn
	js jetstream.JetStream
}

func (p *publisher) close() {
	if p.nc != nil {
		p.nc.Close()
	}
}

func (p *publisher) connect(c *Client) error {
	p.close()
	var err error
	p.nc, p.js, err = c.connect()
	return err
}

func (c *Client) run() {
	if err := c.publish(); err != nil {
		c.setError(err)
		if c.ctx.Err() == nil {
			c.logger.Error("Log publisher stopped with unconfirmed entries", "error", err)
		}
	}
	c.cancel()
	c.stopAccepting()
	// Release payloads still queued if publishing stopped before a full drain.
	for range c.queue {
	}
	c.reportRejections(nil)
	close(c.done)
}

func (c *Client) publish() error {
	var p publisher
	defer p.close()
	queue := c.queue
	window := make([]publication, 0, maxPendingPublications)
	var pendingBytes int
	for queue != nil || len(window) != 0 {
		if err := c.ctx.Err(); err != nil {
			return err
		}

		var input <-chan []byte
		// Reserve room for the largest possible entry before removing one
		// from the queue, so there is no extra payload waiting off-queue.
		if len(window) < maxPendingPublications && pendingBytes+logevent.MaxPayloadBytes <= maxPendingBytes {
			input = queue
		}
		var ack <-chan *jetstream.PubAck
		var failed <-chan error
		if len(window) > 0 {
			ack, failed = window[0].future.Ok(), window[0].future.Err()
		}

		var retry bool
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		case <-ack:
			pendingBytes -= len(window[0].payload)
			window = slices.Delete(window, 0, 1)
		case err := <-failed:
			window[0].future, window[0].err = nil, err
			retry = true
		case payload, ok := <-input:
			if !ok {
				queue = nil
				continue
			}
			if c.dropWhenFull {
				c.admission.Lock()
				c.queuedBytes -= len(payload)
				c.admission.Unlock()
			}
			if p.nc == nil {
				if err := p.connect(c); err != nil {
					return err
				}
			}
			key := logevent.MessageKey(c.config.Topic, payload)
			entry := publication{payload: payload, id: hex.EncodeToString(key[:])}
			entry.send(p.js, c.config.Topic)
			window = append(window, entry)
			pendingBytes += len(payload)
			retry = entry.err != nil
		}
		if retry {
			if err := c.retry(&p, window); err != nil {
				return err
			}
			window, pendingBytes = window[:0], 0
		}
	}
	return nil
}

// Pause new sends while settling the current window, then resend only the
// unconfirmed entries in admission order. JetStream may already have stored
// later entries even if an earlier one needs another attempt.
func (c *Client) retry(p *publisher, window []publication) error {
	for delay := 250 * time.Millisecond; ; delay = min(delay*2, 5*time.Second) {
		unconfirmed := window[:0]
		var firstErr error
		for _, entry := range window {
			err := entry.err
			if entry.future != nil {
				select {
				case <-c.ctx.Done():
					return c.ctx.Err()
				case <-entry.future.Ok():
					continue
				case err = <-entry.future.Err():
				}
			}
			var apiErr *jetstream.APIError
			if errors.Is(err, nats.ErrMaxPayload) || (errors.As(err, &apiErr) && apiErr.ErrorCode == jsErrMessageTooLarge) {
				return fmt.Errorf("broker rejected the payload size: %w", err)
			}
			if firstErr == nil {
				firstErr = err
			}
			// Entries after an immediate send failure may still be unsent.
			entry.future, entry.err = nil, nil
			unconfirmed = append(unconfirmed, entry)
		}
		clear(window[len(unconfirmed):])
		window = unconfirmed
		if len(window) == 0 {
			return nil
		}

		c.logger.Warn("Retrying unconfirmed log publications", "count", len(window), "error", firstErr, "retry_in", delay.String())
		if err := wait(c.ctx, delay); err != nil {
			return err
		}
		if p.nc.IsClosed() {
			if err := p.connect(c); err != nil {
				return err
			}
		}
		for i := range window {
			if err := c.ctx.Err(); err != nil {
				return err
			}
			window[i].send(p.js, c.config.Topic)
			if window[i].err != nil {
				break
			}
		}
	}
}

func (c *Client) connect() (*nats.Conn, jetstream.JetStream, error) {
	options := []nats.Option{
		nats.UserCredentialBytes([]byte(c.config.Creds)),
		nats.Timeout(connectTimeout),
		nats.SetCustomDialer(&contextDialer{ctx: c.ctx}),
		// nats.go's own lookup cannot be canceled; DialContext resolves instead.
		nats.SkipHostLookup(),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		// Only the bounded queue and publication window own retryable data.
		nats.ReconnectBufSize(-1),
		nats.FlusherTimeout(publishTimeout),
		nats.NoCallbacksAfterClientClose(),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				c.logger.Warn("Disconnected from NATS", "error", err)
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			c.logger.Debug("Reconnected to NATS")
		}),
		nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
			c.logger.Error("NATS error", "error", err)
		}),
	}
	for {
		if err := c.ctx.Err(); err != nil {
			return nil, nil, err
		}
		nc, err := nats.Connect(c.config.GetNatsUrl(), options...)
		if err == nil {
			js, err := jetstream.New(nc, jetstream.WithPublishAsyncMaxPending(maxPendingPublications), jetstream.WithPublishAsyncTimeout(publishTimeout))
			if err != nil {
				nc.Close()
				return nil, nil, fmt.Errorf("initialize JetStream: %w", err)
			}
			c.logger.Debug("Connected to NATS", "address", c.config.GetNatsUrl(), "instance", c.instanceID)
			return nc, js, nil
		}
		if c.ctx.Err() != nil {
			return nil, nil, c.ctx.Err()
		}
		c.logger.Warn("Failed to connect to NATS, retrying", "error", err, "retry_in", "2s")
		if err := wait(c.ctx, 2*time.Second); err != nil {
			return nil, nil, err
		}
	}
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
