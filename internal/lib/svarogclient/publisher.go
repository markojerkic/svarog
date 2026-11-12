package svarogclient

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/markojerkic/svarog/internal/lib/logevent"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	connectTimeout = 2 * time.Second
	publishTimeout = 5 * time.Second
	// JetStream's stream max_msg_size rejection cannot succeed on retry.
	jsErrMessageTooLarge jetstream.ErrorCode = 10054
)

func (c *Client) run() {
	var nc *nats.Conn
	var js jetstream.JetStream
	defer func() {
		if nc != nil {
			nc.Close()
		}
		c.cancel()
		c.stopAccepting()
		close(c.done)
	}()

	for payload := range c.queue {
		for {
			if err := c.ctx.Err(); err != nil {
				c.setError(err)
				return
			}
			if nc == nil || nc.IsClosed() {
				var err error
				nc, js, err = c.connect()
				if err != nil {
					c.setError(err)
					return
				}
			}
			err := c.publish(nc, js, payload)
			if errors.Is(err, nats.ErrConnectionClosed) {
				// Keep the current payload when a connection closes permanently.
				continue
			}
			if err != nil {
				c.setError(err)
				c.logger.Error("Log publisher stopped with unconfirmed entries", "error", err)
				return
			}
			break
		}
	}
}

func (c *Client) connect() (*nats.Conn, jetstream.JetStream, error) {
	options := []nats.Option{
		nats.UserCredentialBytes([]byte(c.config.Creds)),
		nats.Timeout(connectTimeout),
		nats.SetCustomDialer(&contextDialer{ctx: c.ctx}),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		// Only the bounded queue and its current payload own retryable data.
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
			js, err := jetstream.New(nc)
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

func (c *Client) publish(nc *nats.Conn, js jetstream.JetStream, payload []byte) error {
	sum := logevent.MessageKey(c.config.Topic, payload)
	id := hex.EncodeToString(sum[:])
	delay := 250 * time.Millisecond
	for {
		if err := c.ctx.Err(); err != nil {
			return err
		}
		if nc.IsClosed() {
			return nats.ErrConnectionClosed
		}
		ctx, cancel := context.WithTimeout(c.ctx, publishTimeout)
		_, err := js.Publish(ctx, c.config.Topic, payload, jetstream.WithMsgID(id))
		cancel()
		if err == nil {
			return nil
		}
		if c.ctx.Err() != nil {
			return c.ctx.Err()
		}
		var apiErr *jetstream.APIError
		if errors.Is(err, nats.ErrMaxPayload) || (errors.As(err, &apiErr) && apiErr.ErrorCode == jsErrMessageTooLarge) {
			return fmt.Errorf("broker rejected the payload size: %w", err)
		}
		c.logger.Warn("Failed to publish log line, retrying", "error", err, "retry_in", delay.String())
		if err := wait(c.ctx, delay); err != nil {
			return err
		}
		delay = min(delay*2, 5*time.Second)
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
