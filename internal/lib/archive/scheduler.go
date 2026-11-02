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

func RunScheduler(ctx context.Context, service *Service, interval time.Duration) {
	if interval <= 0 {
		slog.Info("Archive scheduler disabled")
		return
	}
	ticker := time.NewTicker(schedulerPoll)
	defer ticker.Stop()
	for {
		if _, _, err := service.RunDue(ctx, interval); err != nil && ctx.Err() == nil {
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
// schedules each one interval later. locked is false when another instance is
// running a pass. Per-client failures are logged and retried sooner than the
// interval instead of failing the pass.
func (a *Service) RunDue(ctx context.Context, interval time.Duration) (processed int, locked bool, err error) {
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
			a.schedule(ctx, scope.ClientID, schedulerRetryDelay)
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
				a.schedule(ctx, scope.ClientID, interval)
			}
		}
		return nil
	})
	return processed, locked, err
}

func (a *Service) schedule(ctx context.Context, clientID uuid.UUID, delay time.Duration) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := sqlc.New(a.pool).ScheduleArchivePolicy(ctx, sqlc.ScheduleArchivePolicyParams{
		ClientID: clientID, DelaySeconds: int64(delay / time.Second),
	}); err != nil {
		slog.Error("Couldn't schedule archive policy", "clientId", clientID, "error", err)
	}
}
