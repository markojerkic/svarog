package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type fetchRetry struct {
	failures int
	delay    time.Duration
	at       time.Time
}

// fetchFailed keeps ingest running through broker restarts, leadership moves,
// reconnects and missing responders. A closed connection, an invalid request
// and a confirmed missing or incompatible durable stop ingest instead; the
// durable is never recreated, so its cursor cannot be reset here.
func (s *Service) fetchFailed(ctx context.Context, retry *fetchRetry, err error) error {
	if permanentFetchError(err) {
		return fmt.Errorf("fetch ingest messages: %w", err)
	}
	retry.failures++
	retry.delay = min(max(2*retry.delay, fetchRetryMin), fetchRetryMax)
	retry.at = time.Now().Add(retry.delay)
	slog.Warn("Log fetch failed; keeping fetched deliveries and retrying", "error", err, "failures", retry.failures, "retry_in", retry.delay)
	if errors.Is(err, jetstream.ErrConsumerDeleted) || retry.failures%inspectFetchFailures == 0 {
		return s.inspectConsumer(ctx)
	}
	return nil
}

func permanentFetchError(err error) bool {
	return errors.Is(err, nats.ErrConnectionClosed) || errors.Is(err, jetstream.ErrConnectionClosed) ||
		errors.Is(err, nats.ErrConnectionDraining) || errors.Is(err, nats.ErrBadSubscription) ||
		errors.Is(err, nats.ErrInvalidArg) || errors.Is(err, jetstream.ErrInvalidOption) ||
		errors.Is(err, jetstream.ErrBadRequest)
}

// inspectConsumer turns repeated pull failures into an incident only when the
// broker confirms that log-processor is gone or no longer fits bounded pulls.
func (s *Service) inspectConsumer(ctx context.Context) error {
	infoCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	info, err := s.consumer.Info(infoCtx)
	if errors.Is(err, jetstream.ErrConsumerNotFound) || errors.Is(err, jetstream.ErrStreamNotFound) {
		return fmt.Errorf("LOGS log-processor durable is missing; incident requires operator inspection, not automatic recreation: %w", err)
	}
	if err != nil {
		slog.Warn("Could not inspect the log durable; retrying", "error", err)
		return nil
	}
	if err := validateConsumer(info.Config); err != nil {
		return fmt.Errorf("log-processor changed while ingest was running: %w", err)
	}
	return nil
}
