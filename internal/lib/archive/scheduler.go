package archive

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

const (
	schedulerPoll       = time.Minute
	schedulerRetryDelay = 15 * time.Minute
)

func RunScheduler(ctx context.Context, service *Service) {
	ticker := time.NewTicker(schedulerPoll)
	defer ticker.Stop()
	for {
		if _, _, err := service.RunDue(ctx); err != nil && ctx.Err() == nil {
			slog.Error("Archive scheduler pass failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RunDue archives every due client while holding the scheduler lock, then
// schedules each one its policy's number of weeks later. locked is false when
// another instance is running a pass. Per-client failures are logged and retried
// after 15 minutes instead of waiting for the next full policy interval.
func (a *Service) RunDue(ctx context.Context) (processed int, locked bool, err error) {
	if a.store == nil {
		return 0, false, nil
	}
	locked, err = store.WithArchiveSchedulerLock(ctx, a.pool, func(conn *pgx.Conn) error {
		due, err := sqlc.New(conn).ListDueArchivePolicies(ctx)
		if err != nil {
			return fmt.Errorf("list due archive policies: %w", err)
		}
		for _, row := range due {
			if err := ctx.Err(); err != nil {
				return err
			}
			scope := Scope{ProjectID: row.ProjectID, ClientID: row.ClientID}
			// A crash mid-build retries after the delay rather than on every tick.
			retryAt, err := a.scheduleRetry(ctx, scope.ClientID)
			if errors.Is(err, pgx.ErrNoRows) {
				// The policy was disabled or rescheduled since the due list was read.
				continue
			}
			if err != nil {
				return err
			}
			processed++
			result, err := a.Create(ctx, scope)
			switch {
			case err != nil:
				if !errors.Is(err, context.Canceled) {
					slog.Error("Scheduled archiving failed", "clientId", scope.ClientID, "error", err)
				}
			case result.Outcome == SkippedBusy:
				slog.Info("Scheduled archiving postponed; client is busy", "clientId", scope.ClientID)
			default:
				if result.Outcome == Created {
					slog.Info("Scheduled archive created", "clientId", scope.ClientID, "logs", result.Archive.LogCount)
				}
				a.schedule(ctx, scope.ClientID, retryAt)
			}
		}
		return nil
	})
	return processed, locked, err
}

func (a *Service) scheduleRetry(ctx context.Context, clientID uuid.UUID) (time.Time, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	next, err := sqlc.New(a.pool).RetryArchivePolicy(ctx, sqlc.RetryArchivePolicyParams{
		ClientID: clientID, DelaySeconds: int64(schedulerRetryDelay / time.Second),
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("schedule archive retry for %s: %w", clientID, err)
	}
	return next, nil
}

func (a *Service) schedule(ctx context.Context, clientID uuid.UUID, expectedRunAt time.Time) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	// A policy edit during the build owns its new deadline; do not overwrite it.
	if err := sqlc.New(a.pool).ScheduleArchivePolicy(ctx, sqlc.ScheduleArchivePolicyParams{
		ClientID: clientID, ExpectedRunAt: expectedRunAt,
	}); err != nil {
		slog.Error("Couldn't schedule archive policy", "clientId", clientID, "error", err)
	}
}
