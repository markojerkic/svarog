package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/svaroglog"
)

func run() (result error) {
	interval, err := time.ParseDuration(envOr("DEMO_INTERVAL", "1s"))
	if err != nil || interval <= 0 {
		return errors.New("DEMO_INTERVAL must be a positive duration, for example 1s or 250ms")
	}
	appName := envOr("DEMO_APP_NAME", "demo-native")

	instanceID := envOr("SVAROG_INSTANCE_ID", appName+"-"+uuid.NewString())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	stopOnCancel := context.AfterFunc(ctx, stop)
	defer stopOnCancel()
	closeLogging, err := svaroglog.Setup(ctx,
		svaroglog.WithInstanceID(instanceID),
		svaroglog.WithLevel(slog.LevelDebug),
	)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, closeLogging())
	}()
	slog.SetDefault(slog.Default().With("app", appName, "integration", "native"))

	generateLogs(ctx, interval)
	return nil
}

func generateLogs(ctx context.Context, interval time.Duration) {
	slog.Info("Native demo started", "interval", interval)
	requestLogger := slog.With("component", "checkout").WithGroup("request")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for id := 0; ; id++ {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if ctx.Err() != nil {
			return
		}
		switch id % 5 {
		case 0:
			slog.Info("Order created", "order_id", id, "amount", 49.99)
		case 1:
			requestLogger.Warn("Payment gateway is slow", "order_id", id, "elapsed_ms", 1200)
		case 2:
			slog.Error("Payment declined", "order_id", id, "reason", "insufficient funds")
		case 3:
			slog.Debug("Cache refreshed", "entries", 42)
		case 4:
			log.Printf("Health check passed: request_id=%d", id)
		}
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
